//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/cucumber/godog"
)

// popupTitle is the top edge of a popup: a rule with the title inset (spec §6.5).
var popupTitle = regexp.MustCompile(`^─ (keys|checks) `)

// keyLabels are the labels the keys popup shows for keys, used to count its columns.
var keyLabels = map[string]bool{
	"1–3": true, "?": true, "!": true, "r": true, "q": true, "Esc": true,
	"c": true, "o": true, "n": true, "w": true, "e": true, "d": true, "p": true, "Enter": true,
}

// popup is the open popup as drawn: its title and the rows below it down to the status bar.
type popup struct {
	title string
	top   int // the screen row of the title
	edge  string
	rows  []string
	lines []string // the whole screen
}

// openPopup finds the popup on screen, or reports there is none.
func openPopup(lines []string) (popup, bool) {
	for y, line := range lines {
		if match := popupTitle.FindStringSubmatch(line); match != nil {
			return popup{title: match[1], top: y, edge: line, rows: lines[y+1 : len(lines)-1], lines: lines}, true
		}
	}

	return popup{}, false
}

// expectPopup waits for the popup of the given title (keys or checks) to be open or closed.
func (w *world) expectPopup(title string, open bool) error {
	var failure error
	err := w.expectScreenLines(func(lines []string) error {
		p, found := openPopup(lines)
		switch {
		case open && (!found || p.title != title):
			failure = fmt.Errorf("the %s popup is not open", title)
		case !open && found && p.title == title:
			failure = fmt.Errorf("the %s popup is unexpectedly open", title)
		default:
			failure = nil
		}

		return failure
	})

	return err
}

// withPopup runs check on the open popup.
func (w *world) withPopup(check func(p popup) error) error {
	return w.expectScreenLines(func(lines []string) error {
		p, found := openPopup(lines)
		if !found {
			return errors.New("no popup is open")
		}

		return check(p)
	})
}

func (w *world) expectPopupContains(title string, texts []string) error {
	if err := w.expectPopup(title, true); err != nil {
		return err
	}

	return w.withPopup(func(p popup) error {
		for _, text := range texts {
			if !strings.Contains(strings.Join(p.rows, "\n"), text) {
				return fmt.Errorf("the popup does not show %q", text)
			}
		}

		return nil
	})
}

func quotedList(list string) []string {
	matches := quoted.FindAllStringSubmatch(list, -1)
	texts := make([]string, len(matches))
	for i, match := range matches {
		texts[i] = match[1]
	}

	return texts
}

// countKeyLabels is how many key labels a popup row shows, one per column.
func countKeyLabels(row string) int {
	count := 0
	for field := range strings.FieldsSeq(row) {
		if keyLabels[field] {
			count++
		}
	}

	return count
}

func registerPopupSteps(sc *godog.ScenarioContext, w *world) {
	registerPopupVisibilitySteps(sc, w)
	registerPopupGeometrySteps(sc, w)
}

func registerPopupVisibilitySteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see the keys popup listing ((?:"[^"]*" ?)+)$`, func(list string) error {
		return w.expectPopupContains("keys", quotedList(list))
	})
	step(sc, `^I see ((?:"[^"]*" ?)+) in the keys popup$`, func(list string) error {
		return w.expectPopupContains("keys", quotedList(list))
	})
	step(sc, `^I do not see the keys popup$`, func() error { return w.expectPopup("keys", false) })
	step(sc, `^I see the checks panel$`, func() error { return w.expectPopup("checks", true) })
	step(sc, `^I do not see the checks panel$`, func() error { return w.expectPopup("checks", false) })
	step(sc, `^the view content is still visible above the (?:popup|panel)$`, func() error {
		return w.withPopup(func(p popup) error {
			if first := p.lines[0]; !strings.Contains(first, "1 dash") {
				return fmt.Errorf("the tab bar is covered: %q", first)
			}

			return nil
		})
	})
}

func registerPopupGeometrySteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the (?:popup|panel) spans the full width$`, func() error {
		return w.withPopup(func(p popup) error {
			if got := ansi.StringWidth(p.edge); got != w.width {
				return fmt.Errorf("the popup edge is %d cells wide, want %d", got, w.width)
			}

			return nil
		})
	})
	step(sc, `^the (?:popup|panel) is no taller than half the screen$`, func() error {
		return w.withPopup(func(p popup) error {
			if height := len(p.rows) + 1; height > w.height/2 {
				return fmt.Errorf("the popup is %d rows tall, want at most %d", height, w.height/2)
			}

			return nil
		})
	})
	step(sc, `^the popup uses at most (\d+) columns$`, func(most int) error {
		return w.withPopup(func(p popup) error {
			if columns := countKeyLabels(p.rows[0]); columns > most {
				return fmt.Errorf("the first popup row has %d columns, want at most %d: %q", columns, most, p.rows[0])
			}

			return nil
		})
	})
}
