//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// expectCarry checks the carry transaction of a year file: the last one when closing, the first when opening.
func (w *world) expectCarry(file, kind, amount, account string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(filepath.Join(filepath.Dir(w.journal), file))
		if err != nil {
			return err
		}
		blocks := blocksOf(string(content))
		if len(blocks) == 0 {
			return fmt.Errorf("%s has no transaction", file)
		}
		block := blocks[0]
		if kind == "closing" {
			block = blocks[len(blocks)-1]
		}
		header, _, _ := strings.Cut(block, "\n")
		if !strings.Contains(header, kind+" balance") || !strings.Contains(header, "konto:carry") {
			return fmt.Errorf("%s: %q is no %s carry", file, header, kind)
		}
		if !strings.Contains(block, account+"  "+amount) {
			return fmt.Errorf("%s: %q does not post %s to %s", file, block, amount, account)
		}
		return nil
	})
}

func registerCarrySteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^"([^"]*)" ends with a closing balance of "([^"]*)" on "([^"]*)"$`, func(file, amount, account string) error {
		return w.expectCarry(file, "closing", amount, account)
	})
	step(sc, `^"([^"]*)" starts with an opening balance of "([^"]*)" on "([^"]*)"$`, func(file, amount, account string) error {
		return w.expectCarry(file, "opening", amount, account)
	})
}
