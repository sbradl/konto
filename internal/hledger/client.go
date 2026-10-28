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

// runner is the single seam to the hledger process (spec §4.3): it runs
// hledger with args and returns its standard output and standard error.
type runner func(ctx context.Context, args ...string) (stdout []byte, stderr string, err error)

// inputRunner is the seam for hledger reading journal text from standard input.
type inputRunner func(ctx context.Context, input string, args ...string) (stdout []byte, stderr string, err error)

// Client asks hledger about one journal, made of one file or several (runJournal says how they are passed).
type Client struct {
	run       runner
	runInput  inputRunner
	files     func() []string                             // the files making up the journal, asked for on every call
	tempRules func(rules string) (path string, err error) // the seam for the CSV rules file
}

// callTimeout is how long one hledger call may take before it is killed and fails: generous, since a large
// journal takes about a second a call (ADR 0013), but a hung hledger must not leave a view waiting forever.
const callTimeout = 2 * time.Minute

// New returns a Client that runs the hledger binary bin on the journal made of the files named by files.
func New(bin string, files func() []string) *Client {
	return &Client{
		run: execRunner(bin, callTimeout), runInput: execInputRunner(bin, callTimeout),
		files: files, tempRules: osRules,
	}
}

func execRunner(bin string, timeout time.Duration) runner {
	return func(ctx context.Context, args ...string) ([]byte, string, error) {
		return binary{bin, timeout}.run(ctx, nil, args)
	}
}

func execInputRunner(bin string, timeout time.Duration) inputRunner {
	return func(ctx context.Context, input string, args ...string) ([]byte, string, error) {
		return binary{bin, timeout}.run(ctx, strings.NewReader(input), args)
	}
}

// binary is the hledger executable and how long one call of it may take.
type binary struct {
	path    string
	timeout time.Duration
}

// run runs the binary with args and stdin, killing it once the timeout has passed; it returns stdout and
// stderr.
func (b binary) run(ctx context.Context, stdin io.Reader, args []string) (stdout []byte, stderr string, err error) {
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
	out, stderr, err := c.runInput(ctx, text, append([]string{"-f", "-"}, args...)...)
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
		return c.run(ctx, append([]string{"-f", files[0]}, args...)...)
	}
	var journal strings.Builder
	for _, file := range files {
		journal.WriteString("include " + file + "\n")
	}
	return c.runInput(ctx, journal.String(), append([]string{"-f", "-"}, args...)...)
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
			Date: date, Description: entry.Description, Comment: strings.TrimSpace(entry.Comment), Postings: postings,
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
	rulesPath, err := c.tempRules(rules)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(rulesPath) }()
	out, stderr, err := c.run(ctx, "print", "-f", csvPath, "--rules-file", rulesPath, "-O", "json")
	if err != nil {
		return nil, failure(stderr, err, firstMessageLine, "hledger converting "+csvPath)
	}
	return readTransactions(out, nil)
}

// tempFile is a file just created for the CSV rules.
type tempFile interface {
	io.WriteCloser
	Name() string
}

// writeRules puts the rules in a new temporary file, whose path it returns; a file it could not
// fill is removed again.
func writeRules(create func() (tempFile, error), rules string) (string, error) {
	file, err := create()
	if err != nil {
		return "", fmt.Errorf("writing CSV rules: %w", err)
	}
	_, writeErr := io.WriteString(file, rules)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("writing CSV rules: %w", err)
	}
	return file.Name(), nil
}

// osRules writes the CSV rules to a temporary file on disk.
func osRules(rules string) (string, error) {
	return writeRules(func() (tempFile, error) { return os.CreateTemp("", "konto-*.rules") }, rules)
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
