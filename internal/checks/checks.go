// Package checks is the use case that runs every journal check: each hledger check as
// its own invocation, plus konto's own checks over the loaded Ledger (spec §4.6).
package checks

import (
	"context"
	"fmt"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/parallel"
)

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
	checks, own := domain.HledgerChecks(), domain.KontoChecks()

	issues, err := r.runHledger(ctx, checks)
	if err != nil {
		return domain.CheckRun{}, err
	}

	run := domain.CheckRun{Total: len(checks) + len(own), Issues: failed(issues)}
	for _, check := range own {
		run.Issues = append(run.Issues, check(ledger)...)
	}

	return run, nil
}

// failed is the issues of the hledger checks that found one, dropping those that passed.
func failed(issues []*domain.CheckIssue) []domain.CheckIssue {
	var found []domain.CheckIssue
	for _, issue := range issues {
		if issue != nil {
			found = append(found, *issue)
		}
	}

	return found
}

// runHledger runs hledger's checks side by side: the issue each found, nil where it passed.
func (r *Runner) runHledger(ctx context.Context, checks []domain.Check) ([]*domain.CheckIssue, error) {
	issues := make([]*domain.CheckIssue, len(checks))
	runs := make([]func() error, len(checks))
	for i, check := range checks {
		runs[i] = func() (err error) {
			if issues[i], err = r.gw.Check(ctx, check.Name); err != nil {
				return fmt.Errorf("running check %s: %w", check.Name, err)
			}

			return nil
		}
	}

	return issues, parallel.RunAll(runs...) //nolint:wrapcheck // each run names its check
}

type renderer interface {
	Render(changeset domain.Changeset) (string, error)
}

type textChecker interface {
	CheckText(ctx context.Context, text string, names []string) error
}

// Verifier checks a prepared changeset before it is written, as it is: the journal as it would read afterwards goes to
// hledger on standard input, and only the checks that make a journal wrong (errors, not warnings) count.
type Verifier struct {
	render renderer
	gw     textChecker
}

// NewVerifier returns a Verifier that renders changesets through render and checks them through gw.
func NewVerifier(render renderer, gw textChecker) *Verifier { return &Verifier{render: render, gw: gw} }

// Verify returns why the journal would be wrong after the changeset, or nil. The changeset is the one that will
// be written, already holding what else the write brings (the carry), so it is rendered without adding to it.
func (v *Verifier) Verify(ctx context.Context, changeset domain.Changeset) error {
	text, err := v.render.Render(changeset)
	if err != nil {
		return fmt.Errorf("preparing the write: %w", err)
	}

	if err := v.gw.CheckText(ctx, text, blockingCheckNames()); err != nil {
		return fmt.Errorf("the journal would fail its checks: %w", err)
	}

	return nil
}

// blockingCheckNames names the hledger checks whose failure makes a journal wrong: the errors, not the warnings.
func blockingCheckNames() []string {
	var blocking []string
	for _, check := range domain.HledgerChecks() {
		if check.Severity == domain.SeverityErr {
			blocking = append(blocking, check.Name)
		}
	}

	return blocking
}
