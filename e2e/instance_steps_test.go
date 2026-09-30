//go:build e2e

package e2e

import "github.com/cucumber/godog"

// startAnother starts a further konto on the scenario's journal, leaving the
// running one in place; later steps talk to the newest instance.
func (w *world) startAnother() error {
	w.earlier = append(w.earlier, w.s)
	w.s = nil
	return w.start()
}

func registerInstanceSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^konto is (?:already )?running on it$`, func() error { return w.start() })
	sc.Step(`^I start (?:a second )?konto on the same journal$`, w.startAnother)
}
