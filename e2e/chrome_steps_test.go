//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
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

// setTerminalSize sets the size konto starts at, or resizes it if already running.
func (w *world) setTerminalSize(width, height int) error {
	w.width, w.height = width, height
	if w.s == nil {
		return nil
	}
	if err := w.s.resize(width, height); err != nil {
		return err
	}
	w.s.settle(w.suite.timeout)
	return nil
}

func (w *world) expectScreenLines(check func(lines []string) error) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	var failure error
	s.waitFor(w.suite.timeout, func(string) bool {
		failure = check(s.screenLines())
		return failure == nil
	})
	if failure != nil {
		return fmt.Errorf("%w\n%s", failure, s.screenText())
	}
	return nil
}

func checkLineWidths(width int) func(lines []string) error {
	return func(lines []string) error {
		for i, line := range lines {
			if got := ansi.StringWidth(line); got > width {
				return fmt.Errorf("line %d is %d cells wide, want at most %d", i, got, width)
			}
		}
		return nil
	}
}

func checkHeight(height int) func(lines []string) error {
	return func(lines []string) error {
		if len(lines) != height || strings.TrimSpace(lines[0]) == "" {
			return fmt.Errorf("screen has %d lines and starts with %q, want %d lines from the tab bar down",
				len(lines), lines[0], height)
		}
		return nil
	}
}

func checkStatusBarLast(lines []string) error {
	last := strings.TrimSpace(lines[len(lines)-1])
	for _, tab := range tabNames {
		if strings.HasPrefix(last, tab.name) {
			return nil
		}
	}
	return fmt.Errorf("last line is %q, want the status bar", last)
}

func registerLayoutSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^every line of the screen is at most (\d+) cells wide$`, func(width int) error {
		return w.expectScreenLines(checkLineWidths(width))
	})
	step(sc, `^the screen has exactly (\d+) lines$`, func(height int) error {
		return w.expectScreenLines(checkHeight(height))
	})
	step(sc, `^the status bar is on the last line$`, func() error { return w.expectScreenLines(checkStatusBarLast) })
}

var quoted = regexp.MustCompile(`"([^"]*)"`)

// registerFeatureSteps binds every view's and the chrome's own phrasings, then the tab bar's.
func registerFeatureSteps(sc *godog.ScenarioContext, w *world) {
	registerLayoutSteps(sc, w)
	registerStatusBarSteps(sc, w)
	registerThemeSteps(sc, w)
	registerPopupSteps(sc, w)
	registerAccountsPanelSteps(sc, w)
	registerChecksSteps(sc, w)
	registerAccountChangeSteps(sc, w)
	registerStatCardSteps(sc, w)
	registerRecentSteps(sc, w)
	registerUpcomingSteps(sc, w)
	registerSpendSteps(sc, w)
	registerPanelSteps(sc, w)
	registerChromeExtraSteps(sc, w)
	registerReportsSteps(sc, w)
	registerScheduledSteps(sc, w)
	registerScheduledDeleteSteps(sc, w)
	registerImportSteps(sc, w)
	registerCategoriseSteps(sc, w)
	registerImportWriteSteps(sc, w)
	registerStagingSteps(sc, w)
	registerFormatSteps(sc, w)
	registerForecastSteps(sc, w)
	registerScheduledFormSteps(sc, w)
	registerLeftoverSteps(sc, w)
	registerInvariantSteps(sc, w)
	step(sc, `^the active tab is "([^"]*)"$`, w.expectActiveTab)
	step(sc, `^the active tab is unchanged$`, func() error { return w.expectActiveTab(w.tabAtBaseline) })
	step(sc, `^the tab bar shows the journal path and "(\d+)" transactions on the right$`, func(count string) error {
		return w.expectLine(0, "tab bar", count, func(line, count string) bool {
			last := tabNames[len(tabNames)-1]
			tabsEnd := strings.Index(line, last.key+" "+last.name) + len(last.key+" "+last.name)
			rest := line[tabsEnd:]
			return strings.Contains(rest, w.journal) && strings.Contains(rest, count)
		})
	})
	step(sc, `^the tab bar does not show a check status$`, func() error {
		return w.expectLine(0, "tab bar", "●", func(line, marker string) bool { return !strings.Contains(line, marker) })
	})
}

func registerRefreshSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I append this to the journal externally:$`, func(text *godog.DocString) error {
		f, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, werr := f.WriteString("\n" + text.Content + "\n")
		return errors.Join(werr, f.Close())
	})
	step(sc, `^I append a transaction to the journal externally$`, func() error {
		f, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		const txn = "\n2026-09-30 Newly added\n    assets:checking    1,00 EUR\n    equity:opening-balances\n"
		_, werr := f.WriteString(txn)
		return errors.Join(werr, f.Close())
	})
	step(sc, `^the new transaction appears after refresh$`, func() error {
		return w.expectLine(0, "tab bar", fmt.Sprintf("%d txns", w.transactionsAtStart()+1), strings.Contains)
	})
	step(sc, `^the status bar shows a spinner$`, func() error {
		return w.expectLine(-1, "status bar", spinner, strings.Contains)
	})
	step(sc, `^the status bar shows an error indicator$`, func() error {
		return w.expectLine(-1, "status bar", "✗", strings.Contains)
	})
	step(sc, `^(?:the current panels stay rendered|the previous balances are still shown)$`, func() error {
		return w.expectLine(0, "tab bar", fmt.Sprintf("%d txns", w.transactionsAtStart()), strings.Contains)
	})
}

// datedLine opens a transaction.
var datedLine = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}`)

// transactionsAtStart is how many transactions the journal files held when konto started.
func (w *world) transactionsAtStart() int {
	return len(datedLine.FindAllString(allJournalText(w.filesAtStart), -1))
}

func registerStatusBarSteps(sc *godog.ScenarioContext, w *world) {
	registerRefreshSteps(sc, w)
	step(sc, `^the status bar hint lists ((?:"[^"]*" ?)+)$`, func(list string) error {
		for _, match := range quoted.FindAllStringSubmatch(list, -1) {
			if err := w.expectLine(-1, "status bar", match[1], strings.Contains); err != nil {
				return err
			}
		}
		return nil
	})
}

func registerThemeSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^no cell sets a background colour except the selected row and the popup$`, func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		s.settle(w.suite.timeout)
		for colour := range s.backgrounds() {
			if colour != selectionBackground {
				return fmt.Errorf("a cell has background %d; only the selection background %d is allowed",
					colour, selectionBackground)
			}
		}
		return nil
	})
}
