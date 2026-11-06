package hledger

import (
	"context"
	"errors"

	"github.com/sbradl/konto/internal/domain"
)

// Check runs one named `hledger check` and returns the first failure it reports, or nil
// when the check passes. A failure without a location in its message is an error.
func (c *Client) Check(ctx context.Context, name string) (*domain.CheckIssue, error) {
	_, err := c.query(ctx, "check", name)
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
	_, err := c.queryText(ctx, text, append([]string{"check"}, names...)...)
	if failure, ok := errors.AsType[domain.JournalError](err); ok && failure.Issue.Message != "" {
		return errors.New(failure.Issue.Message) // where in the text it failed means nothing to the user
	}

	return err
}
