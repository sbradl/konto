package accounts

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type fakeRules struct {
	rules map[string][]string
	err   error
}

func (f fakeRules) RulesReferencing(account string) ([]string, error) { return f.rules[account], f.err }

var today = func() time.Time { return time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC) }

func TestCloseStampsTodayOnAnUnusedZeroBalanceAccount(t *testing.T) {
	got, err := NewService(fakeRules{}, today).Close(domain.Account{Name: "assets:spare"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.Changeset{Edits: []domain.Edit{
		domain.SetAccountTag{Account: "assets:spare", Key: "konto:closed", Value: "2026-09-30"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changeset = %+v, want %+v", got, want)
	}
}

func TestCloseIsRefusedWhileARuleStillUsesTheAccount(t *testing.T) {
	rules := fakeRules{rules: map[string][]string{"assets:spare": {"Top-up"}}}

	_, err := NewService(rules, today).Close(domain.Account{Name: "assets:spare"})

	var refusal domain.RefusalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Error(), "still used by: Top-up") {
		t.Errorf("error = %v, want a refusal naming the rule", err)
	}
}

func TestCloseReportsAFailureToReadTheRules(t *testing.T) {
	_, err := NewService(fakeRules{err: errors.New("unreadable")}, today).Close(domain.Account{Name: "assets:spare"})

	if err == nil || !strings.Contains(err.Error(), "assets:spare") || !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("error = %v, want one naming the account and carrying the cause", err)
	}
}

func TestReopenPlansRemovingTheClosedTag(t *testing.T) {
	closed := domain.Account{Name: "assets:old", Closed: today()}

	got, err := NewService(fakeRules{}, today).Reopen(closed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.Changeset{Edits: []domain.Edit{domain.RemoveAccountTag{Account: "assets:old", Key: "konto:closed"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changeset = %+v, want %+v", got, want)
	}
	if _, err := NewService(fakeRules{}, today).Reopen(domain.Account{Name: "assets:open"}); !errors.Is(err, domain.ErrNotClosed) {
		t.Errorf("reopening an open account: error = %v, want ErrNotClosed", err)
	}
}
