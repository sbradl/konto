package hledger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// runner is the single seam to the hledger process (spec §4.3): it runs
// hledger with args and returns its standard output and standard error.
type runner func(ctx context.Context, args ...string) (stdout []byte, stderr string, err error)

// Client asks hledger about one journal. Every call names the journal with -f.
type Client struct {
	run     runner
	journal string
}

// New returns a Client that runs the hledger binary bin on the journal at journalPath.
func New(bin, journalPath string) *Client {
	return newClient(execRunner(bin), journalPath)
}

func newClient(run runner, journalPath string) *Client {
	return &Client{run: run, journal: journalPath}
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
	Date        string `json:"tdate"`
	Description string `json:"tdescription"`
	Postings    []struct {
		Account string       `json:"paccount"`
		Amounts []jsonAmount `json:"pamount"`
	} `json:"tpostings"`
}

// Print returns every transaction in the journal.
func (c *Client) Print(ctx context.Context) ([]domain.Transaction, error) {
	out, err := c.query(ctx, "print", "-O", "json")
	if err != nil {
		return nil, err
	}
	var raw []jsonTransaction
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("reading hledger print output: %w", err)
	}
	transactions := make([]domain.Transaction, len(raw))
	for i, entry := range raw {
		date, err := time.Parse(time.DateOnly, entry.Date)
		if err != nil {
			return nil, fmt.Errorf("reading hledger print output: %w", err)
		}
		postings, err := entry.postings()
		if err != nil {
			return nil, fmt.Errorf("reading hledger print output: %w", err)
		}
		transactions[i] = domain.Transaction{Date: date, Description: entry.Description, Postings: postings}
	}
	return transactions, nil
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
		postings[i] = domain.Posting{Account: posting.Account, Amounts: amounts}
	}
	return postings, nil
}
