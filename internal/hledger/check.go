package hledger

import (
	"context"
	"errors"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// Check runs one named `hledger check` and returns the first failure it reports, or nil
// when the check passes. A failure without a location in its message is an error.
func (c *Client) Check(ctx context.Context, name string) (*domain.CheckIssue, error) {
	_, err := c.queryMessage(ctx, firstParagraph, "check", name)
	if err == nil {
		return nil, nil //nolint:nilnil // nil issue and nil error is the passing check
	}
	if failure, ok := errors.AsType[domain.JournalError](err); ok {
		issue := failure.Issue
		issue.Check, issue.Severity = name, domain.SeverityOf(name)
		return &issue, nil
	}
	return nil, err
}

// CheckText runs the named checks over journal text given on standard input; the first failure is the
// error, in hledger's words. Nothing on disk is read or written for the text itself.
func (c *Client) CheckText(ctx context.Context, text string, names []string) error {
	_, stderr, err := c.runInput(ctx, text, append([]string{"-f", "-", "check"}, names...)...)
	if err != nil {
		_, message, _ := strings.Cut(stderr, "\n") // the first line only says where
		return errors.New(firstParagraph(message))
	}
	return nil
}
