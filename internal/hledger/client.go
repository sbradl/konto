package hledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// runner is the single seam to the hledger process (spec §4.3): it runs hledger with args, reading stdin
// (nil for none), and returns its standard output and standard error.
type runner func(ctx context.Context, stdin io.Reader, args ...string) (stdout []byte, stderr string, err error)

// Client asks hledger about one journal, made of one file or several (runJournal says how they are passed).
type Client struct {
	run   runner
	files func() []string // the files making up the journal, asked for on every call
}

// callBinary is the hledger binary at path with how long one call may take before it is killed and fails:
// generous, since a large journal takes about a second a call (ADR 0013), but a hung hledger must not leave a
// view waiting forever.
func callBinary(path string) binary { return binary{path, 2 * time.Minute} }

// New returns a Client that runs the hledger binary bin on the journal made of the files named by files.
func New(bin string, files func() []string) *Client {
	return &Client{run: callBinary(bin).run, files: files}
}

// binary is the hledger executable and how long one call of it may take.
type binary struct {
	path    string
	timeout time.Duration
}

// run runs the binary with args and stdin, killing it once the timeout has passed; it returns stdout and
// stderr.
func (b binary) run(ctx context.Context, stdin io.Reader, args ...string) (stdout []byte, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()
	var captured bytes.Buffer
	// path is the hledger binary konto was configured with; args are built by Client, never by the user.
	cmd := exec.CommandContext(ctx, b.path, args...) //nolint:gosec // see above
	cmd.Stdin, cmd.Stderr = stdin, &captured
	stdout, err = cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("hledger did not answer within %s: %w", b.timeout, ctx.Err())
	}
	return stdout, captured.String(), err
}

// query runs the hledger command args[0] against the client's journal.
func (c *Client) query(ctx context.Context, args ...string) ([]byte, error) {
	out, stderr, err := c.runJournal(ctx, args)
	if err != nil {
		return nil, failure(stderr, err, messageOf(args[0]), "hledger "+args[0])
	}
	return out, nil
}

// queryText is query on journal text given on standard input instead of the client's journal.
func (c *Client) queryText(ctx context.Context, text string, args ...string) ([]byte, error) {
	out, stderr, err := c.run(ctx, strings.NewReader(text), append([]string{"-f", "-"}, args...)...)
	if err != nil {
		return nil, failure(stderr, err, messageOf(args[0]), "hledger "+args[0])
	}
	return out, nil
}

// messageOf is how a located failure of the hledger command explains itself: a check over several lines,
// anything else on its first line.
func messageOf(command string) func(text string) string {
	if command == "check" {
		return firstParagraph
	}
	return firstMessageLine
}

// failure is why a run of hledger failed: the located issue when stderr names one, read with messageOf, else
// what failed, the cause and hledger's own words.
func failure(stderr string, err error, messageOf func(string) string, what string) error {
	if issue, ok := parseIssue(stderr, messageOf); ok {
		return domain.JournalError{Issue: issue}
	}
	return fmt.Errorf("%s: %w: %s", what, err, strings.TrimSpace(stderr))
}

// runJournal runs hledger on the journal. A single file is named with -f. Several files go as one journal on
// standard input that includes them: hledger checks balance assertions per -f file but across an include, so
// only an include sees the carry-over and every earlier posting of an account.
func (c *Client) runJournal(ctx context.Context, args []string) (stdout []byte, stderr string, err error) {
	files := c.files()
	if len(files) == 1 {
		return c.run(ctx, nil, append([]string{"-f", files[0]}, args...)...)
	}
	var journal strings.Builder
	for _, file := range files {
		journal.WriteString("include " + file + "\n")
	}
	return c.run(ctx, strings.NewReader(journal.String()), append([]string{"-f", "-"}, args...)...)
}

// jsonTransaction is the part of `hledger print -O json` konto reads.
type jsonTransaction struct {
	Date        string     `json:"tdate"`
	Description string     `json:"tdescription"`
	Comment     string     `json:"tcomment"`
	Tags        [][]string `json:"ttags"`
	SourcePos   []struct {
		Name string `json:"sourceName"`
		Line int    `json:"sourceLine"`
	} `json:"tsourcepos"`
	Postings []struct {
		Account string       `json:"paccount"`
		Amounts []jsonAmount `json:"pamount"`
		Tags    [][]string   `json:"ptags"`
	} `json:"tpostings"`
}

// Print returns every transaction in the journal.
func (c *Client) Print(ctx context.Context) ([]domain.Transaction, error) {
	return c.transactions(ctx, nil, "print", "-O", "json")
}

// generatedTag marks the transactions hledger projects from periodic rules.
const generatedTag = "_generated-transaction"

// Upcoming returns the occurrences of the periodic rules from begin up to, not including, until, as hledger
// projects them. The forecast period is always explicit: its default starts after the latest regular transaction.
func (c *Client) Upcoming(ctx context.Context, begin, until time.Time) ([]domain.Transaction, error) {
	return c.transactions(ctx, jsonTransaction.isGenerated, "print", span{begin, until}.forecast(), "-O", "json")
}

func (raw jsonTransaction) isGenerated() bool {
	return slices.ContainsFunc(raw.Tags, func(tag []string) bool { return len(tag) > 0 && tag[0] == generatedTag })
}

// transactions runs a `print` and reads its transactions, only those keep accepts when it is given.
func (c *Client) transactions(
	ctx context.Context, keep func(jsonTransaction) bool, args ...string,
) ([]domain.Transaction, error) {
	out, err := c.query(ctx, args...)
	if err != nil {
		return nil, err
	}
	return readTransactions(out, keep)
}

// readTransactions maps `hledger print -O json` to transactions, only those keep accepts when it is given.
func readTransactions(out []byte, keep func(jsonTransaction) bool) ([]domain.Transaction, error) {
	var raw []jsonTransaction
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("reading hledger print output: %w", err)
	}
	transactions := make([]domain.Transaction, 0, len(raw))
	for _, entry := range raw {
		if keep != nil && !keep(entry) {
			continue
		}
		date, err := time.Parse(time.DateOnly, entry.Date)
		if err != nil {
			return nil, fmt.Errorf("reading hledger print output: %w", err)
		}
		postings, err := entry.postings()
		if err != nil {
			return nil, fmt.Errorf("reading hledger print output: %w", err)
		}
		transaction := domain.Transaction{
			Date: date, Description: entry.Description, Comment: strings.TrimSpace(entry.Comment),
			Tags: tagMap(entry.Tags), Postings: postings,
		}
		if len(entry.SourcePos) > 0 {
			transaction.Source = domain.SourcePos{File: entry.SourcePos[0].Name, Line: entry.SourcePos[0].Line}
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

// ConvertCSV converts the bank export at csvPath to transactions with the given CSV rules, as hledger
// reads them: the rules decide every field, the account of each side and the comments.
func (c *Client) ConvertCSV(ctx context.Context, csvPath, rules string) ([]domain.Transaction, error) {
	file, err := os.CreateTemp("", "konto-*.rules")
	if err == nil { // one check for each way the file can fail: creating, writing or closing it
		defer func() { _ = os.Remove(file.Name()) }()
		_, err = file.WriteString(rules)
		err = errors.Join(err, file.Close())
	}
	if err != nil {
		return nil, fmt.Errorf("writing CSV rules: %w", err)
	}
	out, stderr, err := c.run(ctx, nil, "print", "-f", csvPath, "--rules-file", file.Name(), "-O", "json")
	if err != nil {
		return nil, failure(stderr, err, firstMessageLine, "hledger converting "+csvPath)
	}
	return readTransactions(out, nil)
}

// postings are the transaction's postings with their amounts.
func (raw jsonTransaction) postings() ([]domain.Posting, error) {
	postings := make([]domain.Posting, len(raw.Postings))
	for i, posting := range raw.Postings {
		amounts, err := amounts(posting.Amounts)
		if err != nil {
			return nil, err
		}
		postings[i] = domain.Posting{Account: posting.Account, Amounts: amounts, Tags: tagMap(posting.Tags)}
	}
	return postings, nil
}

// tagMap is hledger's [[key, value], …] as a map; nil when there are no tags.
func tagMap(tags [][]string) map[string]string {
	var named map[string]string
	for _, tag := range tags {
		if len(tag) != tagCells {
			continue
		}
		if named == nil {
			named = map[string]string{}
		}
		named[tag[0]] = tag[1]
	}
	return named
}

const tagCells = 2 // [key, value]
