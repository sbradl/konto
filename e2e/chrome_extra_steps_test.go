//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

var rowPosition = regexp.MustCompile(`\b\d+/\d+\b`)

// expectIndicatorColour checks the colour of the check indicator's dot in the status bar.
func (w *world) expectIndicatorColour(want int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	line, err := w.statusLine()
	if err != nil {
		return err
	}
	before, _, found := strings.Cut(line, "●")
	if !found {
		return fmt.Errorf("no ● in the status bar %q", line)
	}
	y := len(s.screenLines()) - 1
	if got := s.cellColour(len([]rune(before)), y); got != want {
		return fmt.Errorf("the indicator is coloured %d, want %d", got, want)
	}
	return nil
}

func (w *world) expectNoModeBadge() error {
	line, err := w.statusLine()
	if err != nil {
		return err
	}
	for _, badge := range []string{"NORMAL", "INSERT", "COMMAND", "VISUAL"} {
		if strings.Contains(line, badge) {
			return fmt.Errorf("status bar %q shows the mode badge %s", line, badge)
		}
	}
	return nil
}

func (w *world) expectPanelTitleColour(want int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.waitFor(w.suite.timeout, func(screen string) bool { return strings.Contains(screen, "ACCOUNTS") })
	seen := s.colours("ACCOUNTS")
	if len(seen) == 0 {
		return errors.New("no panel title on screen")
	}
	if seen[0] != want {
		return fmt.Errorf("a panel title is coloured %d, want %d", seen[0], want)
	}
	return nil
}

// expectSelectedRowColours finds a highlighted cell in the body and checks its foreground.
func (w *world) expectSelectedRowColours(bg, fg int) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	s.settle(w.suite.timeout)
	lines := s.screenLines()
	for y := 2; y < len(lines)-1; y++ {
		for x := range []rune(lines[y]) {
			if s.cellBackground(x, y) == bg && strings.TrimSpace(string([]rune(lines[y])[x:x+1])) != "" {
				if got := s.cellColour(x, y); got != fg {
					return fmt.Errorf("selected cell (%d,%d) has foreground %d, want %d", x, y, got, fg)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("no cell with background %d in the body", bg)
}

func (w *world) expectFillsTerminal(width, height int) error {
	return w.expectScreenLines(func(lines []string) error {
		if len(lines) != height {
			return fmt.Errorf("the screen has %d rows, want %d", len(lines), height)
		}
		if !strings.Contains(lines[0], "1 dash") || !strings.Contains(lines[height-1], "? keys") {
			return fmt.Errorf("tab bar or status bar missing at %d×%d", width, height)
		}
		for y, line := range lines {
			if n := len([]rune(line)); n > width {
				return fmt.Errorf("row %d is %d wide, want at most %d", y, n, width)
			}
		}
		return nil
	})
}

// useDefaultEditor leaves VISUAL and EDITOR unset and puts a recording "vi" first on PATH.
func (w *world) useDefaultEditor() error {
	stub := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + w.editorLog() + "\"\n"
	if err := os.WriteFile(filepath.Join(w.binDir, "vi"), []byte(stub), 0o755); err != nil {
		return err
	}
	w.env["EDITOR"], w.env["VISUAL"] = "", ""
	return nil
}

func (w *world) expectStatusBarContents() error {
	return w.expectLine(-1, "status bar", "", func(line, _ string) bool {
		for _, part := range []string{"dash", "c close", "● ", "? keys"} {
			if !strings.Contains(line, part) {
				return false
			}
		}
		return true
	})
}

func registerChromeExtraSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the status bar shows the view name, a hint, the check status and "\? keys"$`, w.expectStatusBarContents)
	step(sc, `^the indicator is coloured (\d+)$`, w.expectIndicatorColour)
	step(sc, `^the status bar does not show a mode badge$`, w.expectNoModeBadge)
	step(sc, `^a panel title is coloured (\d+)$`, w.expectPanelTitleColour)
	step(sc, `^the selected row has background (\d+) and foreground (\d+)$`, w.expectSelectedRowColours)
	step(sc, `^the dashboard fills (\d+)×(\d+)$`, w.expectFillsTerminal)
	step(sc, `^neither VISUAL nor EDITOR is set$`, w.useDefaultEditor)
	step(sc, `^a default editor is started with "(.*)"$`, w.expectEditorCall)
	step(sc, `^the Accounts cursor is on row (\d+)$`, func(row int) error {
		return w.expectLine(-1, "status bar", fmt.Sprintf("%d/", row), func(line, text string) bool {
			return strings.Contains(line, " "+text)
		})
	})
	step(sc, `^the status bar shows a row position$`, func() error {
		return w.expectLine(-1, "status bar", "", func(line, _ string) bool { return rowPosition.MatchString(line) })
	})
	step(sc, `^the status bar does not show a row position$`, func() error {
		return w.expectLine(-1, "status bar", "", func(line, _ string) bool { return !rowPosition.MatchString(line) })
	})
}
