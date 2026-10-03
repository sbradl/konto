// Package checks is the use case that runs every journal check: each hledger check as
// its own invocation, plus konto's own checks over the loaded Ledger (spec §4.6).
package checks

import (
	"context"
	"fmt"

	"github.com/sbradl/konto/internal/domain"
)

// hledgerChecks are every `hledger check`, always, in the order the panel reports them.
func hledgerChecks() []string {
	return []string{
		"parseable", "autobalanced", "assertions", "balanced", "commodities", "accounts",
		"ordereddates", "tags", "payees", "recentassertions", "uniqueleafnames",
	}
}

// ownChecks is how many checks konto runs itself over the Ledger.
const ownChecks = 1

type gateway interface {
	Check(ctx context.Context, name string) (*domain.CheckIssue, error)
}

// Runner runs the checks.
type Runner struct {
	gw gateway
}

// NewRunner returns a Runner that runs hledger's checks through gw.
func NewRunner(gw gateway) *Runner { return &Runner{gw: gw} }

// Run runs every check against the journal and the loaded ledger. hledger reports only the
// first failure per check, so each contributes at most one issue; konto's own checks list all.
func (r *Runner) Run(ctx context.Context, ledger domain.Ledger) (domain.CheckRun, error) {
	run := domain.CheckRun{Total: len(hledgerChecks()) + ownChecks}
	for _, name := range hledgerChecks() {
		issue, err := r.gw.Check(ctx, name)
		if err != nil {
			return domain.CheckRun{}, fmt.Errorf("running check %s: %w", name, err)
		}
		if issue != nil {
			run.Issues = append(run.Issues, *issue)
		}
	}
	run.Issues = append(run.Issues, ledger.UnclassifiedIssues()...)
	return run, nil
}

type renderer interface {
	Render(changeset domain.Changeset) (string, error)
}

type textChecker interface {
	CheckText(ctx context.Context, text string, names []string) error
}

// Verifier checks a changeset before it is written: the journal as it would read afterwards goes to
// hledger on standard input, and only the checks that make a journal wrong (errors, not warnings) count.
type Verifier struct {
	render renderer
	gw     textChecker
}

// NewVerifier returns a Verifier that renders changesets through render and checks them through gw.
func NewVerifier(render renderer, gw textChecker) *Verifier { return &Verifier{render: render, gw: gw} }

// Verify returns why the journal would be wrong after the changeset, or nil.
func (v *Verifier) Verify(ctx context.Context, changeset domain.Changeset) error {
	text, err := v.render.Render(changeset)
	if err != nil {
		return fmt.Errorf("preparing the write: %w", err)
	}
	var blocking []string
	for _, name := range hledgerChecks() {
		if domain.SeverityOf(name) == domain.SeverityErr {
			blocking = append(blocking, name)
		}
	}
	if err := v.gw.CheckText(ctx, text, blocking); err != nil {
		return fmt.Errorf("the journal would fail its checks: %w", err)
	}
	return nil
}
