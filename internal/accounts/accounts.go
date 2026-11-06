// Package accounts is the use case that closes and reopens accounts (ADR 0012): it gathers
// what the Account aggregate needs, lets the aggregate decide, and returns the changeset to write.
package accounts

import (
	"fmt"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type ruleSource interface {
	RulesReferencing(account string) ([]string, error)
}

// Service plans account changes.
type Service struct {
	rules ruleSource
	today func() time.Time
}

// NewService returns a Service that asks rules which periodic rules use an account and stamps
// closings with today.
func NewService(rules ruleSource, today func() time.Time) *Service {
	return &Service{rules: rules, today: today}
}

// Close plans closing the account, or says why it cannot be closed.
func (s *Service) Close(account domain.Account) (domain.Changeset, error) {
	rules, err := s.rules.RulesReferencing(account.Name)
	if err != nil {
		return domain.Changeset{}, fmt.Errorf("closing %s: %w", account.Name, err)
	}

	return account.Close(s.today(), rules) //nolint:wrapcheck // the aggregate's refusal is already user-facing
}

// Reopen plans reopening the account, or says why it cannot be reopened.
func (*Service) Reopen(account domain.Account) (domain.Changeset, error) {
	return account.Reopen() //nolint:wrapcheck // the aggregate's refusal is already user-facing
}
