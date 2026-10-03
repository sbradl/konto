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
