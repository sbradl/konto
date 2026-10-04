//go:build e2e

package e2e

import (
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

func (w *world) expectPanels(titles ...string) error {
	for _, title := range titles {
		if err := w.expectScreen(true, "╭─ "+title+" "); err != nil {
			return err
		}
	}
	return nil
}

func (w *world) expectNoLineWiderThan(width int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	for y, line := range s.screenLines() {
		if n := len([]rune(strings.TrimRight(line, " "))); n > width {
			return fmt.Errorf("line %d is %d cells wide, want at most %d: %q", y, n, width, line)
		}
	}
	return nil
}

func (w *world) expectRoundedBorders() error {
	if err := w.expectScreen(true, "╭"); err != nil {
		return err
	}
	for _, square := range []string{"┌", "┐", "┘"} { // not "└": the chart axis uses it
		if err := w.expectScreen(false, square); err != nil {
			return err
		}
	}
	return nil
}

func registerPanelSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see the "([^"]*)" panel$`, func(title string) error { return w.expectPanels(title) })
	step(sc, `^I see the "([^"]*)" and "([^"]*)" panels$`, func(a, b string) error { return w.expectPanels(a, b) })
	step(sc, `^the layout has no line wider than (\d+) cells$`, w.expectNoLineWiderThan)
	step(sc, `^the top border of the "([^"]*)" panel reads "([^"]*)"$`, func(_, border string) error {
		return w.expectScreen(true, border)
	})
	step(sc, `^panel borders use rounded box-drawing characters$`, w.expectRoundedBorders)
}
