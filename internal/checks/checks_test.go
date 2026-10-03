package checks

import (
	"context"
	"errors"
	"reflect"
	"slices"
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

type fakeRender struct {
	text string
	err  error
}

func (f fakeRender) Render(domain.Changeset) (string, error) { return f.text, f.err }

type fakeTextChecker struct {
	err   error
	text  string
	names []string
}

func (f *fakeTextChecker) CheckText(_ context.Context, text string, names []string) error {
	f.text, f.names = text, names
	return f.err
}

func TestVerifyChecksTheRenderedJournalForErrorsOnly(t *testing.T) {
	gw := &fakeTextChecker{}

	err := NewVerifier(fakeRender{text: "journal"}, gw).Verify(context.Background(), domain.Changeset{})

	if err != nil || gw.text != "journal" {
		t.Errorf("err = %v, text = %q", err, gw.text)
	}
	if want := []string{"parseable", "autobalanced", "assertions", "balanced"}; !slices.Equal(gw.names, want) {
		t.Errorf("checks = %v, want %v", gw.names, want)
	}
}

func TestVerifyReportsAFailedCheck(t *testing.T) {
	gw := &fakeTextChecker{err: errors.New("balance assertion")}

	err := NewVerifier(fakeRender{}, gw).Verify(context.Background(), domain.Changeset{})

	if err == nil || !strings.Contains(err.Error(), "balance assertion") {
		t.Errorf("error = %v", err)
	}
}

func TestVerifyReportsAChangesetThatCannotBeRendered(t *testing.T) {
	gw := &fakeTextChecker{}

	err := NewVerifier(fakeRender{err: errors.New("no such file")}, gw).Verify(context.Background(), domain.Changeset{})

	if err == nil || gw.names != nil {
		t.Errorf("error = %v, checked = %v; want an error and no check", err, gw.names)
	}
}
