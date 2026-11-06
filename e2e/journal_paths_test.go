//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// Journal path precedence (spec §7.1): which source each phrase in the
// "journal path precedence" outline stands for.
const (
	sourceFlag    = "flag"
	sourceEnv     = "env"
	sourceDefault = "home default"
)

func (w *world) sourcePath(source string) string {
	switch source {
	case sourceFlag:
		return filepath.Join(w.dir, "flag.journal")
	case sourceEnv:
		return filepath.Join(w.dir, "env.journal")
	default:
		return filepath.Join(w.home, ".hledger.journal")
	}
}

// startWithSources starts konto with the flag and/or $LEDGER_FILE the phrase names.
func (w *world) startWithSources(invocation string) error {
	var args []string
	if strings.Contains(invocation, "--file") {
		args = []string{"--file", w.sourcePath(sourceFlag)}
	}

	if strings.Contains(invocation, "$LEDGER_FILE") {
		w.env["LEDGER_FILE"] = w.sourcePath(sourceEnv)
	} else {
		w.unset["LEDGER_FILE"] = true
	}

	return w.start(args...)
}

func registerJournalPathSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^journals exist at (.+)$`, func(sources string) error {
		for source := range strings.SplitSeq(sources, ", ") {
			if err := os.WriteFile(w.sourcePath(source), []byte("; "+source+"\n"), 0o644); err != nil {
				return err
			}
		}

		return nil
	})
	step(sc, `^I start konto with (--file flag and \$LEDGER_FILE set|only \$LEDGER_FILE set|nothing set)$`, w.startWithSources)
	step(sc, `^a journal "([^"]*)" at "([^"]*)"$`, func(name, path string) error {
		content, err := os.ReadFile(w.fixture(name + ".journal"))
		if err != nil {
			return err
		}

		return os.WriteFile(filepath.Join(w.dir, path), content, 0o644)
	})
	step(sc, `^the tab bar shows an absolute path ending in "([^"]*)"$`, func(suffix string) error {
		absolute := regexp.MustCompile(`\s/\S*` + regexp.QuoteMeta(suffix) + `(\s|$)`)

		return w.expectLine(0, "tab bar", "", func(line, _ string) bool { return absolute.MatchString(line) })
	})
	step(sc, `^the tab bar shows the path of (flag|env|default)$`, func(winner string) error {
		if winner == "default" {
			winner = sourceDefault
		}
		path := w.sourcePath(winner)

		return w.expectLine(0, "tab bar", path, strings.Contains)
	})
}
