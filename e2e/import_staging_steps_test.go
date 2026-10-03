//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"
)

// expectCursorOn waits for the selected row of the table to hold the description.
func (w *world) expectCursorOn(description string) error {
	return w.expectPanel(rowsTitle, func(rows []recentRow) error {
		selected, err := w.selectedRowIndex(rows)
		if err != nil {
			return err
		}
		if !strings.Contains(rows[selected].text, description) {
			return fmt.Errorf("the selected row is %q, want %q", rows[selected].text, description)
		}
		return nil
	})
}

// appendExternally adds a checking transaction to the journal as another program would.
func (w *world) appendExternally(date, payee, amount string) error {
	text := fmt.Sprintf("\npayee %s\n\n%s %s\n    expenses:groceries  %s EUR\n    assets:checking\n", payee, date, payee, amount)
	file, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (w *world) expectRuleFromDisk(id string) error {
	if err := w.expectStaged(false, "rule "+id); err != nil {
		return err
	}
	return w.expectPanel(rowsTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, "#"+id+" ") {
				return nil
			}
		}
		return fmt.Errorf("no row shows rule #%s", id)
	})
}

func registerStagingSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the cursor is where I left it$`, func() error { return w.expectCursorOn(w.leftRow) })
	sc.Step(`^the table row "([^"]*)" is selected$`, w.expectCursorOn)
	sc.Step(`^I append to the journal externally a checking transaction on (\d{4}-\d{2}-\d{2}) "([^"]*)" of ([\d,]+) EUR$`,
		w.appendExternally)
	sc.Step(`^the row "([^"]*)" is skipped$`, func(name string) error { return w.expectRowState(name, "", "", "skipped") })
	sc.Step(`^I see a y/n prompt about discarding the staged import$`, func() error {
		return w.expectStatusHas(true, []string{"discard the staged import", "(y/n)"})
	})
	sc.Step(`^I see the rule (\d+) was read from categories\.rules$`, w.expectRuleFromDisk)
	sc.Step(`^"w" is blocked only by the (\d+) new rows that need a category$`, func(n int) error {
		if err := w.press("w"); err != nil {
			return err
		}
		return w.expectScreen(true, fmt.Sprintf("%d rows need a category", n))
	})
}
