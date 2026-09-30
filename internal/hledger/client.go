package hledger

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// runner is the single seam to the hledger process (spec §4.3): it runs
// hledger with args and returns its standard output.
type runner func(ctx context.Context, args ...string) ([]byte, error)

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
	return func(ctx context.Context, args ...string) ([]byte, error) {
		// bin is the hledger binary konto was configured with; args are built by Client, never by the user.
		return exec.CommandContext(ctx, bin, args...).Output() //nolint:gosec // see above
	}
}

// query runs hledger against the client's journal.
func (c *Client) query(ctx context.Context, args ...string) ([]byte, error) {
	out, err := c.run(ctx, append([]string{"-f", c.journal}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("hledger %s: %w", args[0], err)
	}
	return out, nil
}

// jsonTransaction is the part of `hledger print -O json` konto reads.
type jsonTransaction struct {
	Date        string `json:"tdate"`
	Description string `json:"tdescription"`
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
	for i, r := range raw {
		date, err := time.Parse(time.DateOnly, r.Date)
		if err != nil {
			return nil, fmt.Errorf("reading hledger print output: %w", err)
		}
		transactions[i] = domain.Transaction{Date: date, Description: r.Description}
	}
	return transactions, nil
}
