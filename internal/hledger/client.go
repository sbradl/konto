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

// Client asks hledger about one journal. Every call names the journal with -f.
type Client struct {
	run       runner
	runInput  inputRunner
	journal   string
	tempRules func(rules string) (path string, err error) // the seam for the CSV rules file
}

// New returns a Client that runs the hledger binary bin on the journal at journalPath.
func New(bin, journalPath string) *Client {
	client := newClient(execRunner(bin), journalPath)
	client.runInput = execInputRunner(bin)
	return client
}

func newClient(run runner, journalPath string) *Client {
	return &Client{run: run, journal: journalPath, tempRules: osRules}
}

func execRunner(bin string) runner {
	return func(ctx context.Context, args ...string) ([]byte, string, error) {
		var stderr bytes.Buffer
		// bin is the hledger binary konto was configured with; args are built by Client, never by the user.
		cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // see above
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		return out, stderr.String(), err
	}
}

func execInputRunner(bin string) inputRunner {
	return func(ctx context.Context, input string, args ...string) ([]byte, string, error) {
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // see execRunner
		cmd.Stdin, cmd.Stderr = strings.NewReader(input), &stderr
		out, err := cmd.Output()
		return out, stderr.String(), err
	}
}

// query runs hledger against the client's journal.
func (c *Client) query(ctx context.Context, args ...string) ([]byte, error) {
	return c.queryMessage(ctx, firstMessageLine, args...)
}

// queryMessage is query with its own reading of a located failure's message.
func (c *Client) queryMessage(ctx context.Context, messageOf func(string) string, args ...string) ([]byte, error) {
	out, stderr, err := c.run(ctx, append([]string{"-f", c.journal}, args...)...)
	if err != nil {
		if issue, ok := parseIssue(stderr, messageOf); ok {
			return nil, domain.JournalError{Issue: issue}
		}
		return nil, fmt.Errorf("hledger %s: %w: %s", args[0], err, strings.TrimSpace(stderr))
	}
	return out, nil
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

// Upcoming returns the occurrences of the periodic rules from first through last, as hledger projects them.
// The forecast period is always explicit: its default starts after the latest regular transaction.
func (c *Client) Upcoming(ctx context.Context, first, last time.Time) ([]domain.Transaction, error) {
	period := first.Format(time.DateOnly) + ".." + last.Format(time.DateOnly)
	return c.transactions(ctx, jsonTransaction.isGenerated, "print", "--forecast="+period, "-O", "json")
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
		return nil, fmt.Errorf("hledger converting %s: %w: %s", csvPath, err, strings.TrimSpace(stderr))
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
		amounts := make(domain.MultiAmount, len(posting.Amounts))
		for j, amount := range posting.Amounts {
			var err error
			if amounts[j], err = amount.amount(); err != nil {
				return nil, err
			}
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
