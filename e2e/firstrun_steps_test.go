//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cucumber/godog"
)

func registerFirstRunSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^no journal exists at "([^"]*)"$`, func(path string) error {
		w.journal = w.expand(path)
		w.rules = filepath.Join(filepath.Dir(w.journal), "categories.rules")
		return removeIfExists(w.journal)
	})
	sc.Step(`^a file "([^"]*)" exists and is empty$`, func(path string) error {
		info, err := os.Stat(w.expand(path))
		if err != nil {
			return err
		}
		if info.Size() != 0 {
			return fmt.Errorf("%s has %d bytes, want an empty file", path, info.Size())
		}
		return nil
	})
	sc.Step(`^no journal is created at "([^"]*)"$`, func(path string) error { return expectAbsent(w.expand(path)) })
	sc.Step(`^I see the resolved journal path$`, func() error { return w.expectScreen(true, w.journal) })
	sc.Step(`^I see "([^"]*)" to create and "([^"]*)" to quit$`, func(create, quit string) error {
		if err := w.expectScreen(true, create+" to create"); err != nil {
			return err
		}
		return w.expectScreen(true, quit+" to quit")
	})
}
