package checks

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

type fakeGateway struct {
	issues map[string]*domain.CheckIssue
	errs   map[string]error
	ran    []string
}

func (f *fakeGateway) Check(_ context.Context, name string) (*domain.CheckIssue, error) {
	f.ran = append(f.ran, name)
	return f.issues[name], f.errs[name]
}

func TestRunRunsEveryHledgerCheckOnceInOrder(t *testing.T) {
	gw := &fakeGateway{}

	run, err := NewRunner(gw).Run(context.Background(), domain.Ledger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"parseable", "autobalanced", "assertions", "balanced", "commodities", "accounts",
		"ordereddates", "tags", "payees", "recentassertions", "uniqueleafnames",
	}
	if !reflect.DeepEqual(gw.ran, want) {
		t.Errorf("checks run = %v, want %v", gw.ran, want)
	}
	if run.Total != 12 || !run.IsClean() {
		t.Errorf("run = %+v, want a clean run of 12 checks", run)
	}
}

func TestRunCollectsFailuresAndKontosOwnIssues(t *testing.T) {
	gw := &fakeGateway{issues: map[string]*domain.CheckIssue{
		"accounts": {Check: "accounts", Severity: domain.SeverityWarn, Line: 10},
		"payees":   {Check: "payees", Severity: domain.SeverityWarn, Line: 9},
	}}
	ledger := domain.Ledger{Accounts: []domain.Account{{Name: "assets:spare"}}}

	run, err := NewRunner(gw).Run(context.Background(), ledger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := make([]string, len(run.Issues))
	for i, issue := range run.Issues {
		checks[i] = issue.Check
	}
	if want := []string{"accounts", "payees", "unclassified"}; !reflect.DeepEqual(checks, want) {
		t.Errorf("issues = %v, want %v", checks, want)
	}
}

func TestRunStopsOnAFailureToRunACheck(t *testing.T) {
	gw := &fakeGateway{errs: map[string]error{"tags": errors.New("exit status 2")}}

	_, err := NewRunner(gw).Run(context.Background(), domain.Ledger{})

	if err == nil || !strings.Contains(err.Error(), "tags") || !strings.Contains(err.Error(), "exit status 2") {
		t.Errorf("error = %v, want one naming the check and carrying the cause", err)
	}
	if got := gw.ran[len(gw.ran)-1]; got != "tags" {
		t.Errorf("last check run = %s, want the run to stop at tags", got)
	}
}
