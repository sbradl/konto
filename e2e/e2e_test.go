//go:build e2e

// Package e2e drives the compiled konto binary through the Gherkin features
// in features/: a real pty, a virtual terminal, the real hledger behind a
// recording shim, one sandbox per scenario. Run with `mise run test:e2e`;
// pass godog flags after `--`, e.g. `-godog.tags=@spec-4.5`.
package e2e

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// Scenarios tagged @wip (not built yet) or @needs-decision (blocked on the
// user) are excluded; everything else must pass with no undefined steps.
var opts = godog.Options{
	Paths:       []string{"../features"},
	Tags:        "~@wip && ~@needs-decision",
	Strict:      true,
	Format:      "pretty",
	Concurrency: 4,
	Randomize:   -1,
}

// A few godog options as plain test flags, e.g.
// `go test -tags e2e ./e2e -args -godog.tags=@spec-4.5 -godog.format=progress`.
func init() {
	flag.StringVar(&opts.Tags, "godog.tags", opts.Tags, "godog tag expression")
	flag.StringVar(&opts.Format, "godog.format", opts.Format, "godog formatter: pretty, progress, junit:<file>, …")
	flag.IntVar(&opts.Concurrency, "godog.concurrency", opts.Concurrency, "scenarios run in parallel")
	flag.Int64Var(&opts.Randomize, "godog.random", opts.Randomize, "scenario order seed; -1 picks one, 0 keeps file order")
}

func TestFeatures(t *testing.T) {
	s, err := newSuite()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(s.buildDir()) })

	o := opts
	o.TestingT = t
	status := godog.TestSuite{
		Name:                "konto",
		ScenarioInitializer: func(sc *godog.ScenarioContext) { initScenario(sc, s) },
		Options:             &o,
	}.Run()
	if status != 0 {
		t.Fatalf("godog exited with status %d", status)
	}
}

func initScenario(sc *godog.ScenarioContext, s *suite) {
	w := &world{}
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		nw, err := newWorld(s)
		if err != nil {
			return ctx, err
		}
		*w = *nw
		return ctx, nil
	})
	sc.StepContext().Before(func(ctx context.Context, st *godog.Step) (context.Context, error) {
		w.markBaseline(string(st.Type))
		return ctx, nil
	})
	sc.After(func(ctx context.Context, scn *godog.Scenario, err error) (context.Context, error) {
		if err != nil {
			w.dump(scn.Name)
		}
		w.close()
		return ctx, nil
	})
	registerSteps(sc, w)
}

// Gherkin step types as godog reports them.
const (
	actionStep  = "Action"
	outcomeStep = "Outcome"
)

// markBaseline snapshots the screen as the first action of a When block
// starts, so "the screen is unchanged" compares against what the user saw
// before acting.
func (w *world) markBaseline(stepType string) {
	if stepType == actionStep && w.lastStepType != actionStep && w.s != nil {
		w.baseline = w.s.screenText()
		w.tabAtBaseline, _ = w.activeTab()
	}
	w.lastStepType = stepType
}
