//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// tabNames are the v1 tabs as the tab bar labels them.
var tabNames = []struct{ key, name string }{{"1", "dash"}, {"2", "import"}, {"3", "sched"}, {"4", "forecast"}, {"5", "reports"}}

// selectionBackground is the ANSI index of the selected-row background (spec §7.2).
const selectionBackground = 8

// activeTab is the tab drawn on the selection background.
func (w *world) activeTab() (string, error) {
	s, err := w.session()
	if err != nil {
		return "", err
	}
	for _, tab := range tabNames {
		if x, y, found := s.find(tab.key + " " + tab.name); found && y == 0 && s.cellBackground(x, y) == selectionBackground {
			return tab.name, nil
		}
	}
	return "", errors.New("no tab is drawn as active:\n" + s.screenText())
}

func (w *world) expectActiveTab(want string) error {
	var got string
	var err error
	deadline := time.Now().Add(w.suite.timeout)
	for time.Now().Before(deadline) {
		if got, err = w.activeTab(); err == nil && got == want {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("active tab is %q, want %q", got, want)
}

func registerChromeSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the active tab is "([^"]*)"$`, w.expectActiveTab)
	sc.Step(`^the active tab is unchanged$`, func() error { return w.expectActiveTab(w.tabAtBaseline) })
	sc.Step(`^the tab bar shows the journal path and "(\d+)" transactions on the right$`, func(count string) error {
		return w.expectLine(0, "tab bar", count, func(line, count string) bool {
			tabsEnd := strings.Index(line, "5 reports") + len("5 reports")
			rest := line[tabsEnd:]
			return strings.Contains(rest, w.journal) && strings.Contains(rest, count)
		})
	})
	sc.Step(`^the tab bar does not show a check status$`, func() error {
		return w.expectLine(0, "tab bar", "●", func(line, marker string) bool { return !strings.Contains(line, marker) })
	})
}
