package hledger

import (
	"context"
	"errors"

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
