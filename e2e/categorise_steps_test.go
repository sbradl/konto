//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

// dialogTitle is the top edge of a categorising dialog: a rule with the title inset, like a popup.
var dialogTitle = regexp.MustCompile(`^─ (categorise|new rule|add to rule|widen rule \d+|category|create account|write) `)

// dialog is the categorising dialog as drawn: its title and the rows below it down to the status bar.
type dialog struct {
	title string
	rows  []string // trimmed, blank rows left out
}

func findDialog(lines []string) (dialog, bool) {
	for y, line := range lines {
		match := dialogTitle.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		d := dialog{title: match[1]}
		for _, row := range lines[y+1 : len(lines)-1] {
			if text := strings.TrimSpace(row); text != "" {
				d.rows = append(d.rows, text)
			}
		}
		return d, true
	}
	return dialog{}, false
}

// withDialog runs check on the open dialog, waiting for it to hold.
func (w *world) withDialog(check func(d dialog) error) error {
	return w.expectScreenLines(func(lines []string) error {
		d, found := findDialog(lines)
		if !found {
			return errors.New("no dialog is open")
		}
		return check(d)
	})
}

// dialogNow is the open dialog on the settled screen right now.
func (w *world) dialogNow() (dialog, error) {
	s, err := w.session()
	if err != nil {
		return dialog{}, err
	}
	w.waitLoaded(s)
	s.settle(w.suite.timeout)
	d, found := findDialog(s.screenLines())
	if !found {
		return dialog{}, errors.New("no dialog is open")
	}
	return d, nil
}

func (w *world) expectDialogTitle(title string) error {
	return w.withDialog(func(d dialog) error {
		if d.title != title {
			return fmt.Errorf("the %q dialog is open, want %q", d.title, title)
		}
		return nil
	})
}

func (w *world) expectNoDialog() error {
	return w.expectScreenLines(func(lines []string) error {
		if d, found := findDialog(lines); found {
			return fmt.Errorf("the %q dialog is open", d.title)
		}
		return nil
	})
}

func (d dialog) holds(text string) bool { return strings.Contains(strings.Join(d.rows, "\n"), text) }

// listRows are the entries of a list dialog: its rows below the filter line.
func (d dialog) listRows() []string {
	if len(d.rows) == 0 {
		return nil
	}
	return d.rows[1:]
}

// selectRow moves the cursor of the table to the row of the description (and date, when given).
func (w *world) selectRow(description, date string) error {
	w.leftRow = description
	if err := w.press("Home"); err != nil {
		return err
	}
	var index int
	if err := w.expectPanel(rowsTitle, func(rows []recentRow) error {
		found, err := importRowIndex(rows, description, date)
		index = found
		return err
	}); err != nil {
		return err
	}
	return w.press(repeat("↓", index)...)
}

// importRowIndex is the table row whose description is exactly the text, else the first that contains it.
func importRowIndex(rows []recentRow, description, date string) (int, error) {
	inDate := func(row recentRow) bool { return date == "" || strings.Contains(row.text, date) }
	for _, pattern := range []string{description + "  ", description} {
		for i, row := range rows {
			if inDate(row) && strings.Contains(row.text, pattern) {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("no row %q (%s)", description, date)
}

// chooseOption moves to the option of the open dialog and presses Enter; "+ create" is the last entry.
// It waits for the dialog to list the option, so a half-drawn frame is not mistaken for the dialog.
func (w *world) chooseOption(option string) error {
	if option == "+ create" {
		if _, err := w.dialogNow(); err != nil {
			return err
		}
		return w.press("End", "Enter")
	}
	index := 0
	err := w.withDialog(func(d dialog) error {
		rows := d.rows
		if d.title != "categorise" {
			rows = d.listRows()
		}
		for i, row := range rows {
			if strings.Contains(row, option) {
				index = i
				return nil
			}
		}
		return fmt.Errorf("the dialog does not offer %q", option)
	})
	if err != nil {
		return err
	}
	return w.press(append(repeat("↓", index), "Enter")...)
}

// pickCategory gives the open picker, or the new-rule form's category, the category by typing its name.
func (w *world) pickCategory(name string) error {
	for range fieldsInForm {
		d, err := w.dialogNow()
		if err != nil {
			return err
		}
		if d.title == "category" {
			break
		}
		if err := w.press("Tab"); err != nil {
			return err
		}
	}
	if err := w.expectDialogTitle("category"); err != nil {
		return err
	}
	if err := w.typeText(name); err != nil {
		return err
	}
	return w.press("Enter")
}

const fieldsInForm = 4

func (w *world) startNewRule(description, category string, onTheFly bool) error {
	for _, step := range []func() error{
		func() error { return w.selectRow(description, "") },
		func() error { return w.press("Enter") },
		func() error { return w.chooseOption("new rule") },
	} {
		if err := step(); err != nil {
			return err
		}
	}
	if !onTheFly {
		return w.pickCategory(category)
	}
	for range fieldsInForm {
		if d, err := w.dialogNow(); err != nil || d.title == "category" {
			break
		}
		if err := w.press("Tab"); err != nil {
			return err
		}
	}
	if err := w.typeText(category); err != nil {
		return err
	}
	return w.press("End", "Enter")
}

func (w *world) typeAfterClearing(text string) error {
	if err := w.press("Ctrl+U"); err != nil {
		return err
	}
	return w.typeText(text)
}

func (w *world) expectDialogRow(label, text string) error {
	return w.withDialog(func(d dialog) error {
		for _, row := range d.rows {
			if strings.Contains(strings.TrimLeft(row, "▸ "), label) && strings.Contains(row, text) {
				return nil
			}
		}
		return fmt.Errorf("no %q row holds %q: %q", label, text, d.rows)
	})
}

// previewOf are the rule lines of the dialog: from the `if` line to the comment2 line.
func previewOf(d dialog) []string {
	for i, row := range d.rows {
		if strings.HasPrefix(row, "if ") {
			return d.rows[i:]
		}
	}
	return nil
}

func (w *world) expectPreview(doc *godog.DocString) error {
	want := strings.Split(strings.TrimSpace(doc.Content), "\n")
	return w.withDialog(func(d dialog) error {
		got := previewOf(d)
		for i, line := range want {
			if i >= len(got) || got[i] != strings.TrimSpace(line) {
				return fmt.Errorf("the rule preview is %q, want %q", got, want)
			}
		}
		return nil
	})
}

func (w *world) expectPreviewLine(check func(lines []string) bool, text string) error {
	return w.withDialog(func(d dialog) error {
		if !check(previewOf(d)) {
			return fmt.Errorf("the rule preview %q does not match %q", previewOf(d), text)
		}
		return nil
	})
}

// groupOf is which group of the picker a row belongs to.
func groupOf(row string) string {
	switch {
	case strings.Contains(row, "(transfer)"):
		return "transfer"
	case strings.HasPrefix(row, "expenses:"):
		return "expenses:*"
	case strings.HasPrefix(row, "income:"):
		return "income:*"
	}
	return "other"
}

func (w *world) expectGroup(position int, want string) error {
	return w.withDialog(func(d dialog) error {
		var groups []string
		for _, row := range d.listRows() {
			if group := groupOf(row); len(groups) == 0 || groups[len(groups)-1] != group {
				groups = append(groups, group)
			}
		}
		if position >= len(groups) || groups[position] != want {
			return fmt.Errorf("the picker's groups are %v, want %q at %d", groups, want, position)
		}
		return nil
	})
}

// stagedLine is the info block's row of what the import has staged.
func (w *world) expectStaged(want bool, text string) error {
	return w.expectPanel(infoTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, "staged") {
				if strings.Contains(row.text, text) != want {
					return fmt.Errorf("the staged line %q: contains %q is %v, want %v", row.text, text, !want, want)
				}
				return nil
			}
		}
		return errors.New("no staged line")
	})
}

func (w *world) expectRefusal(want bool) error {
	if want {
		return w.expectScreen(true, "refused:")
	}
	return w.eventually(func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if strings.Contains(s.screenText(), "refused:") {
			return errors.New("a refusal is shown")
		}
		return nil
	})
}

func registerCategoriseSteps(sc *godog.ScenarioContext, w *world) {
	registerDialogSteps(sc, w)
	registerCategoriseRowSteps(sc, w)
	registerPickerSteps(sc, w)
	registerRuleFormSteps(sc, w)
}

func registerDialogSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I select the row "([^"]*)"$`, func(name string) error { return w.selectRow(name, "") })
	step(sc, `^I select the row "([^"]*)" dated "([^"]*)"$`, w.selectRow)
	step(sc, `^I choose "([^"]*)"$`, w.chooseOption)
	step(sc, `^I see a popup offering exactly ((?:"[^"]*"(?:, )?)+)$`, func(list string) error {
		return w.withDialog(func(d dialog) error {
			want := quotedList(list)
			if d.title != "categorise" || strings.Join(d.rows, "|") != strings.Join(want, "|") {
				return fmt.Errorf("the %q dialog offers %q, want %q", d.title, d.rows, want)
			}
			return nil
		})
	})
	step(sc, `^I do not see the popup$`, w.expectNoDialog)
	step(sc, `^I see the rule list to add to$`, func() error { return w.expectDialogTitle("add to rule") })
	step(sc, `^the row table is still visible$`, func() error { return w.expectScreen(true, "╭─ ROWS ") })
	step(sc, `^I pick the category "([^"]*)"$`, w.pickCategory)
	step(sc, `^I pick the transfer target "([^"]*)"$`, w.pickCategory)
}

func registerCategoriseRowSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the row "([^"]*)" still shows "([^"]*)"$`, func(name, text string) error {
		return w.expectImportRowHolds(name, "", text)
	})
	step(sc, `^the row "([^"]*)" shows target "([^"]*)"$`, func(name, target string) error {
		return w.expectImportRowHolds(name, "", target)
	})
	step(sc, `^the row "([^"]*)" shows "([^"]*)" and rule id "([^"]*)"$`, func(name, mark, id string) error {
		return w.expectImportRowHolds(name, "", mark, "#"+id)
	})
	step(sc, `^the row "([^"]*)" (?:still )?shows rule id "([^"]*)"$`, func(name, id string) error {
		return w.expectImportRowHolds(name, "", "#"+id)
	})
	step(sc, `^the row "([^"]*)" shows "([^"]*)" coloured (\d+)$`, func(name, mark string, colour int) error {
		return w.expectImportRowColour(name, "", mark, colour)
	})
	step(sc, `^the row "([^"]*)" is marked as a transfer$`, func(name string) error {
		return w.expectImportRowHolds(name, "", "⇄")
	})
	step(sc, `^the row dated "([^"]*)" (?:still )?shows "([^"]*)"$`, func(date, text string) error {
		return w.expectImportRowHolds("", date, text)
	})
	step(sc, `^the row table shows the description "([^"]*)"$`, func(text string) error {
		return w.expectImportRowHolds(text, "")
	})
	step(sc, `^the summary reads "([^"]*)"$`, func(text string) error {
		return w.expectPanel(infoTitle, func(rows []recentRow) error {
			for _, row := range rows {
				if strings.Contains(row.text, text) {
					return nil
				}
			}
			return fmt.Errorf("no info row reads %q", text)
		})
	})
	step(sc, `^no rule is staged$`, func() error { return w.expectStaged(false, "rule ") })
	step(sc, `^the rule (\d+) is unchanged in the staged rules$`, func(id string) error {
		return w.expectStaged(false, "rule "+id)
	})
	step(sc, `^the staged unit contains the directive "([^"]*)"$`, func(text string) error { return w.expectStaged(true, text) })
	step(sc, `^the staged net income change is (\d+),(\d+) EUR$`, func(whole, cents string) error {
		return w.expectStaged(true, "net income "+whole+","+cents+" EUR")
	})
	step(sc, `^no new file exists next to the journal$`, w.expectNoStateFiles)
	step(sc, `^I do not see "([^"]*)" change in the reports$`, w.expectReportsUnchangedFor)
}

// expectReportsUnchangedFor shows the reports and checks the account has no staged amount there: the
// journal is not written while categorising, so its balance there is the journal's own.
func (w *world) expectReportsUnchangedFor(account string) error {
	if err := w.press("5"); err != nil {
		return err
	}
	return w.expectScreenLines(func(lines []string) error {
		for _, line := range lines {
			if strings.Contains(line, account) && strings.Contains(line, " 2,00 EUR") {
				return fmt.Errorf("the reports show the staged amount: %q", line)
			}
		}
		return nil
	})
}

func registerPickerSteps(sc *godog.ScenarioContext, w *world) {
	registerPickerListSteps(sc, w)
	registerPickerGroupSteps(sc, w)
}

func registerPickerListSteps(sc *godog.ScenarioContext, w *world) {
	lists := func(want bool, name, suffix string) error {
		return w.withDialog(func(d dialog) error {
			listed := false
			for _, row := range d.listRows() {
				listed = listed || strings.HasPrefix(row, name) && strings.Contains(row, suffix)
			}
			if listed != want {
				return fmt.Errorf("the picker lists %q is %v, want %v: %q", name, listed, want, d.rows)
			}
			return nil
		})
	}
	step(sc, `^the picker lists "([^"]*)"$`, func(name string) error { return lists(true, name, "") })
	step(sc, `^the picker does not list "([^"]*)"$`, func(name string) error { return lists(false, name, "") })
	step(sc, `^the picker lists "([^"]*)" as a transfer target$`, func(name string) error {
		return lists(true, name, "(transfer)")
	})
}

func registerPickerGroupSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the picker's first group is the "([^"]*)" categories$`, func(group string) error {
		return w.expectGroup(0, group)
	})
	step(sc, `^the second group is the "([^"]*)" categories$`, func(group string) error { return w.expectGroup(1, group) })
	step(sc, `^the third group is the other open accounts as transfer targets$`, func() error {
		return w.expectGroup(2, "transfer")
	})
	step(sc, `^the suggested name is "([^"]*)"$`, func(name string) error {
		return w.withDialog(func(d dialog) error {
			if !d.holds("+ create " + name) {
				return fmt.Errorf("the picker does not suggest %q: %q", name, d.rows)
			}
			return nil
		})
	})
	step(sc, `^I choose "\+ create" with an empty name$`, func() error { return w.press("End", "Enter") })
	step(sc, `^I see a refusal mentioning "([^"]*)" or "([^"]*)" or "([^"]*)"$`, func(a, b, c string) error {
		return w.withDialog(func(d dialog) error {
			if !d.holds("refused:") || (!d.holds(a) && !d.holds(b) && !d.holds(c)) {
				return fmt.Errorf("no refusal mentions %q, %q or %q: %q", a, b, c, d.rows)
			}
			return nil
		})
	})
}

func registerRuleFormSteps(sc *godog.ScenarioContext, w *world) {
	registerRuleDraftSteps(sc, w)
	registerRulePreviewSteps(sc, w)
	registerRuleRefusalSteps(sc, w)
	step(sc, `^I pick the rule "([^"]*)"$`, w.pickRule)
	step(sc, `^I edit the added matcher to "([^"]*)"$`, w.typeAfterClearing)
	step(sc, `^the rule list shows only rules whose category contains "([^"]*)"$`, w.expectRuleListShows)
}

func registerRuleDraftSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the pattern input contains "([^"]*)"$`, func(text string) error { return w.expectDialogRow("pattern", text) })
	step(sc, `^I type "([^"]*)" after clearing the pattern$`, w.typeAfterClearing)
	step(sc, `^I start a new rule for "([^"]*)" with category "([^"]*)"$`, func(name, category string) error {
		return w.startNewRule(name, category, false)
	})
	step(sc, `^I start a new rule for "([^"]*)" with category "([^"]*)" created on the fly$`, func(name, category string) error {
		return w.startNewRule(name, category, true)
	})
	step(sc, `^I confirm(?: the rule)?$`, func() error { return w.press("Enter") })
	step(sc, `^I clear the pattern and confirm$`, func() error { return w.press("Ctrl+U", "Enter") })
	step(sc, `^I confirm without choosing a category$`, func() error { return w.press("Enter") })
	step(sc, `^I add the purpose condition "([^"]*)"$`, w.addPurposeCondition)
}

func (w *world) pickRule(id string) error {
	d, err := w.dialogNow()
	if err != nil {
		return err
	}
	for i, row := range d.listRows() {
		if strings.HasPrefix(row, "#"+id+" ") {
			return w.press(append(repeat("↓", i), "Enter")...)
		}
	}
	return fmt.Errorf("the rule list has no rule %s: %q", id, d.rows)
}

func registerRulePreviewSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see the rule preview$`, w.expectPreview)
	step(sc, `^the rule preview contains "([^"]*)"$`, func(text string) error {
		return w.withDialog(func(d dialog) error {
			if !strings.Contains(strings.Join(previewOf(d), "\n"), text) {
				return fmt.Errorf("the rule preview %q lacks %q", previewOf(d), text)
			}
			return nil
		})
	})
	step(sc, `^the rule preview starts with "([^"]*)"$`, func(text string) error {
		return w.expectPreviewLine(func(lines []string) bool { return len(lines) > 0 && strings.HasPrefix(lines[0], text) }, text)
	})
}

func registerRuleRefusalSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see a message that the pattern is required$`, func() error { return w.expectScreen(true, "the pattern is required") })
	step(sc, `^I see a refusal naming "([^"]*)"$`, func(name string) error {
		if err := w.expectRefusal(true); err != nil {
			return err
		}
		return w.expectScreen(true, name)
	})
	step(sc, `^I see no refusal$`, func() error { return w.expectRefusal(false) })
	step(sc, `^saving is blocked$`, func() error { return w.expectScreen(true, "saving is blocked") })
	step(sc, `^I see "([^"]*)" and "([^"]*)"$`, func(a, b string) error {
		if err := w.expectScreen(true, a); err != nil {
			return err
		}
		return w.expectScreen(true, b)
	})
}

func (w *world) expectRuleListShows(text string) error {
	return w.withDialog(func(d dialog) error {
		if len(d.listRows()) == 0 {
			return errors.New("the rule list is empty")
		}
		for _, row := range d.listRows() {
			if !strings.Contains(row, text) {
				return fmt.Errorf("the rule list shows %q", row)
			}
		}
		return nil
	})
}

func (w *world) addPurposeCondition(text string) error {
	if err := w.press("Tab"); err != nil {
		return err
	}
	return w.typeText(text)
}
