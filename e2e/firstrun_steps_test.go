//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

func registerFirstRunSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^no journal exists at "([^"]*)"$`, func(path string) error {
		w.journal = w.expand(path)
		w.rules = filepath.Join(filepath.Dir(w.journal), "categories.rules")

		return removeIfExists(w.journal)
	})
	step(sc, `^a file "([^"]*)" exists and is empty$`, func(path string) error {
		info, err := os.Stat(w.expand(path))
		if err != nil {
			return err
		}

		if info.Size() != 0 {
			return fmt.Errorf("%s has %d bytes, want an empty file", path, info.Size())
		}

		return nil
	})
	step(sc, `^no journal is created at "([^"]*)"$`, func(path string) error { return expectAbsent(w.expand(path)) })
	step(sc, `^I see the resolved journal path$`, func() error { return w.expectScreen(true, w.journal) })
	step(sc, `^I see "([^"]*)" to create and "([^"]*)" to quit$`, func(create, quit string) error {
		if err := w.expectScreen(true, create+" to create"); err != nil {
			return err
		}

		return w.expectScreen(true, quit+" to quit")
	})
}

func registerParseErrorSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^a journal containing "(.*)" on line (\d+)$`, func(text string, line int) error {
		content := strings.Repeat("\n", line-1) + unescape(text) + "\n"

		return os.WriteFile(w.journal, []byte(content), 0o644)
	})
	step(sc, `^I see an "(ERR|WARN)" issue with a "file:line" reference$`, func(severity string) error {
		if err := w.expectScreen(true, severity); err != nil {
			return err
		}

		return w.expectScreenMatch(`\S+:\d+(:\d+)?`)
	})
	step(sc, `^I see an offer to jump to the editor$`, func() error {
		return w.expectScreen(true, "Enter to jump to the editor")
	})
}

// expectScreenMatch waits for the screen to match the pattern.
func (w *world) expectScreenMatch(pattern string) error {
	s, err := w.session()
	if err != nil {
		return err
	}

	re := regexp.MustCompile(pattern)
	if s.waitFor(w.suite.timeout, re.MatchString) {
		return nil
	}

	return fmt.Errorf("screen does not match %q:\n%s", pattern, s.screenText())
}
