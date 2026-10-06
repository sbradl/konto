//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

var formTitle = regexp.MustCompile(`^─ (new|edit) periodic rule `)

// formLabels are the fields of the form, in focus order.
func formLabels() []string {
	return []string{
		"next due date", "frequency", "remaining payments", "description", "amount", "currency",
		"from account", "to account",
	}
}

// formRows are the rows of the open form below its title rule, trimmed, down to the one above the status bar.
func (w *world) formRows() ([]string, error) {
	s, err := w.session()
	if err != nil {
		return nil, err
	}
	lines := s.screenLines()
	for y, line := range lines {
		if !formTitle.MatchString(line) {
			continue
		}
		rows := make([]string, 0, len(lines)-y)
		for _, row := range lines[y+1 : len(lines)-1] {
			rows = append(rows, strings.TrimRight(row, " "))
		}
		return rows, nil
	}
	return nil, errors.New("no form is open")
}

// shownFocus is the label of the focused field, once the form shows one.
func (w *world) shownFocus() (string, error) {
	var focused string
	err := w.onForm(func(rows []string) error {
		if focused = focusedLabel(rows); focused == "" {
			return errors.New("the form shows no focus yet")
		}
		return nil
	})
	return focused, err
}

// waitFocusMovedFrom is best effort: a Tab that moves nothing just times out and the caller reads again.
func (w *world) waitFocusMovedFrom(label string) {
	_ = w.onForm(func(rows []string) error {
		if focusedLabel(rows) == label {
			return errors.New("the focus has not moved")
		}
		return nil
	})
}

// onForm waits for the open form to satisfy check.
func (w *world) onForm(check func(rows []string) error) error {
	return w.eventually(func() error {
		rows, err := w.formRows()
		if err != nil {
			return err
		}
		return check(rows)
	})
}

// fieldRow is the row of the field with the label, without the focus mark.
func fieldRow(rows []string, label string) (string, bool) {
	for _, row := range rows {
		if rest, found := strings.CutPrefix(strings.TrimLeft(row, "▸ "), label); found && (rest == "" || strings.HasPrefix(rest, " ")) {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

// focusedLabel is the label of the field with the focus.
func focusedLabel(rows []string) string {
	for _, row := range rows {
		if rest, found := strings.CutPrefix(strings.TrimSpace(row), "▸"); found {
			for _, label := range formLabels() {
				if strings.HasPrefix(rest, label) {
					return label
				}
			}
		}
	}
	return ""
}

// focusField presses Tab until the field has the focus. A stale frame must not decide: the focus is read once
// the form shows one, and after each Tab the next read waits for it to move.
func (w *world) focusField(label string) error {
	for range len(formLabels()) + 1 {
		focused, err := w.shownFocus()
		if err != nil {
			return err
		}
		if focused == label {
			return nil
		}
		if err := w.press("Tab"); err != nil {
			return err
		}
		w.waitFocusMovedFrom(focused)
	}
	return fmt.Errorf("the focus never reached %q", label)
}

// fillText replaces what a text field holds.
func (w *world) fillText(label, text string) error {
	if err := w.focusField(label); err != nil {
		return err
	}
	if err := w.press("Ctrl+U"); err != nil || text == "" {
		return err
	}
	return w.typeText(text)
}

// pickAccount chooses the account in a picker field by typing its name.
func (w *world) pickAccount(label, account string) error {
	if err := w.fillText(label, account); err != nil {
		return err
	}
	return w.press("Enter")
}

// chooseFrequency moves the frequency dropdown to the choice.
func (w *world) chooseFrequency(name string) error {
	return w.chooseFrom("frequency", name, "←", 7)
}

// chooseFrom picks the entry of a dropdown: all the way left, then right until it is the bracketed one.
func (w *world) chooseFrom(label, name, back string, steps int) error {
	if err := w.focusField(label); err != nil {
		return err
	}
	if err := w.press(repeat(back, steps)...); err != nil {
		return err
	}
	for range steps {
		rows, err := w.formRows()
		if err != nil {
			return err
		}
		if value, _ := fieldRow(rows, label); strings.Contains(value, "["+name+"]") {
			return nil
		}
		if err := w.press("→"); err != nil {
			return err
		}
	}
	return fmt.Errorf("the %s dropdown has no choice %q", label, name)
}

// validForm is what the form is filled with unless a scenario says otherwise.
func validForm() map[string]string {
	return map[string]string{
		"next due date": "2026-10-15", "frequency": "monthly", "description": "Book club", "amount": "12,00",
		"currency": "EUR", "from account": "assets:checking", "to account": "expenses:books",
	}
}

// fillForm fills every field with the value; an empty value leaves the field empty.
func (w *world) fillForm(values map[string]string) error {
	for _, label := range formLabels() {
		value, given := values[label]
		var err error
		switch {
		case !given:
			continue
		case label == "frequency":
			err = w.chooseFrequency(value)
		case label == "currency":
			err = w.chooseFrom("currency", value, "←", 5)
		case label == "from account" || label == "to account":
			if value != "" {
				err = w.pickAccount(label, value)
			}
		default:
			err = w.fillText(label, value)
		}
		if err != nil {
			return fmt.Errorf("filling %s: %w", label, err)
		}
	}
	return nil
}

func (w *world) fillExcept(field, value string) error {
	values := validForm()
	values[field] = value
	return w.fillForm(values)
}

var ruleDescription = regexp.MustCompile(`^I create a rule with next due date ([\d-]+), frequency ([a-z-]+),(?: (\d+) remaining payments,)? ` +
	`description "([^"]*)", amount "([^"]*)", currency "([^"]*)", from "([^"]*)", to "([^"]*)"$`)

// createRule fills the form with the rule and confirms it.
func (w *world) createRule(date, frequency, remaining, description, amount, currency, from, to string) error {
	values := map[string]string{
		"next due date": date, "frequency": frequency, "description": description, "amount": amount,
		"currency": currency, "from account": from, "to account": to,
	}
	if remaining != "" {
		values["remaining payments"] = remaining
	}
	if err := w.fillForm(values); err != nil {
		return err
	}
	return w.press("Ctrl+S")
}

func (w *world) expectFieldHolds(label, text string) error {
	return w.onForm(func(rows []string) error {
		value, found := fieldRow(rows, label)
		if !found || !strings.Contains(strings.ReplaceAll(value, "▏", ""), text) {
			return fmt.Errorf("the field %q holds %q, want it to hold %q", label, value, text)
		}
		return nil
	})
}

func (w *world) expectFieldIs(label, text string) error {
	return w.onForm(func(rows []string) error {
		value, found := fieldRow(rows, label)
		if !found || !strings.Contains(value, "["+text+"]") && strings.ReplaceAll(strings.TrimSpace(value), "▏", "") != text {
			return fmt.Errorf("the field %q is %q, want %q", label, value, text)
		}
		return nil
	})
}

func (w *world) expectField(label string, shown bool) error {
	return w.onForm(func(rows []string) error {
		if _, found := fieldRow(rows, label); found != shown {
			return fmt.Errorf("the field %q shown is %v, want %v", label, found, shown)
		}
		return nil
	})
}

func (w *world) expectFormClosed(closed bool) error {
	return w.eventually(func() error {
		if _, err := w.formRows(); (err != nil) != closed {
			return fmt.Errorf("the form closed is %v, want %v", err != nil, closed)
		}
		return nil
	})
}

func (w *world) expectFormMessage(field string) error {
	return w.onForm(func(rows []string) error {
		for _, row := range rows {
			if strings.Contains(row, field) && strings.Contains(row, "required") {
				return nil
			}
		}
		return fmt.Errorf("no message about the missing %q: %q", field, rows)
	})
}

func (w *world) expectPickerOffers(label, account string, offered bool) error {
	if err := w.focusField(label); err != nil {
		return err
	}
	return w.onForm(func(rows []string) error {
		if got := slices.ContainsFunc(rows, func(row string) bool { return strings.Contains(row, account) }); got != offered {
			return fmt.Errorf("the %s picker offers %q is %v, want %v", label, account, got, offered)
		}
		return nil
	})
}

func (w *world) expectCurrencyChoices() error {
	declared, err := declaredCommodities(w.journalFiles())
	if err != nil {
		return err
	}
	return w.onForm(func(rows []string) error {
		value, _ := fieldRow(rows, "currency")
		for _, name := range declared {
			if !strings.Contains(value, name) {
				return fmt.Errorf("the currency choices %q lack the declared %s", value, name)
			}
		}
		return nil
	})
}

var commodityLine = regexp.MustCompile(`(?m)^commodity [\d.,]+ ([A-Z]{3})$`)

func declaredCommodities(files map[string][]byte) ([]string, error) {
	var names []string
	for _, content := range files {
		for _, match := range commodityLine.FindAllStringSubmatch(string(content), -1) {
			names = append(names, match[1])
		}
	}
	if len(names) == 0 {
		return nil, errors.New("the journal declares no commodity")
	}
	return names, nil
}

func (w *world) expectFrequencyChoices(list string) error {
	return w.onForm(func(rows []string) error {
		value, _ := fieldRow(rows, "frequency")
		at := 0
		for _, name := range quotedTexts(list) {
			found := strings.Index(value[at:], name)
			if found < 0 {
				return fmt.Errorf("the frequency choices %q lack %q in order", value, name)
			}
			at += found + len(name)
		}
		return nil
	})
}

func (w *world) expectFocusOn(label string) error {
	return w.onForm(func(rows []string) error {
		if got := focusedLabel(rows); got != label {
			return fmt.Errorf("the focus is on %q, want %q", got, label)
		}
		return nil
	})
}

// changeAmount edits the amount field of the open form.
func (w *world) changeAmount(amount string) error { return w.fillText("amount", amount) }

// mainJournalText is the main journal as it is now.
func (w *world) mainJournalText() (string, error) {
	content, err := os.ReadFile(w.journal)
	return string(content), err
}

func (w *world) expectJournalHas(text string, want bool) error {
	return w.eventually(func() error {
		if got := strings.Contains(allJournalText(w.journalFiles()), text); got != want {
			return fmt.Errorf("the journal contains %q is %v, want %v", text, got, want)
		}
		return nil
	})
}

func (w *world) expectFileHas(name, text string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(filepath.Join(filepath.Dir(w.journal), name))
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), text) {
			return fmt.Errorf("%s lacks %q", name, text)
		}
		return nil
	})
}

func (w *world) expectMainEndsWithRule() error {
	return w.eventually(func() error {
		text, err := w.mainJournalText()
		if err != nil {
			return err
		}
		blocks := ruleBlocks(text)
		if len(blocks) == 0 || !strings.HasSuffix(strings.TrimRight(text, "\n"), blocks[len(blocks)-1]) {
			return errors.New("the main journal does not end with a rule")
		}
		return nil
	})
}

func (w *world) expectSameLineCount() error {
	return w.eventually(func() error {
		before, now := 0, 0
		for _, content := range w.filesAtStart {
			before += strings.Count(string(content), "\n")
		}
		for _, content := range w.journalFiles() {
			now += strings.Count(string(content), "\n")
		}
		if before != now {
			return fmt.Errorf("the journal has %d lines, had %d", now, before)
		}
		return nil
	})
}

// blockOf is the rule block with the description, as it is written now.
func (w *world) blockOf(description string) (string, error) {
	for _, block := range ruleBlocks(allJournalText(w.journalFiles())) {
		if header, _, _ := strings.Cut(block, "\n"); strings.Contains(header, description) {
			return block, nil
		}
	}
	return "", fmt.Errorf("no rule %q in the journal", description)
}

func (w *world) expectBlockShows(description, text string) error {
	return w.eventually(func() error {
		block, err := w.blockOf(description)
		if err == nil && !strings.Contains(block, text) {
			err = fmt.Errorf("the block of %q is %q, want it to show %q", description, block, text)
		}
		return err
	})
}

func (w *world) expectRuleGone(description string) error {
	return w.eventually(func() error {
		if _, err := w.blockOf(description); err == nil {
			return fmt.Errorf("the rule %q is still in the journal", description)
		}
		return nil
	})
}

// expectOthersUnchanged checks that every rule of the start but the named ones is still written exactly so.
func (w *world) expectOthersUnchanged(names ...string) error {
	return w.eventually(func() error {
		now := allJournalText(w.journalFiles())
		for _, block := range ruleBlocks(allJournalText(w.filesAtStart)) {
			header, _, _ := strings.Cut(block, "\n")
			if slices.ContainsFunc(names, func(name string) bool { return strings.Contains(header, name) }) {
				continue
			}
			if !strings.Contains(now, block) {
				return fmt.Errorf("the rule %q changed", header)
			}
		}
		return nil
	})
}

func (w *world) expectRuleDirectlyBelow(comment string) error {
	return w.eventually(func() error {
		for _, content := range w.journalFiles() {
			lines := strings.Split(string(content), "\n")
			for i, line := range lines[:len(lines)-1] {
				if line == comment && strings.HasPrefix(lines[i+1], "~") {
					return nil
				}
			}
		}
		return fmt.Errorf("%q is not directly above a rule", comment)
	})
}

func (w *world) expectRuleListRow(description, date string) error {
	return w.expectPanel(rulesTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, description) && strings.Contains(row.text, date) {
				return nil
			}
		}
		return fmt.Errorf("no rule row shows %q with %s", description, date)
	})
}

func (w *world) expectOccurrencesFor(description string, table *godog.Table) error {
	if err := w.selectRule(description); err != nil {
		return err
	}
	return w.expectOccurrences(table, true)
}

func (w *world) expectHledgerPrints(description, first, second string) error {
	return w.eventually(func() error {
		out, err := hledgerOn(w.suite.realHledger, w.journal, "print", "--forecast=2026-10-01..2028-01-01",
			"desc:"+description).CombinedOutput()
		if err != nil {
			return fmt.Errorf("hledger print: %s", out)
		}
		for _, date := range []string{first, second} {
			if !strings.Contains(string(out), date+" "+description) {
				return fmt.Errorf("hledger prints no %s occurrence on %s:\n%s", description, date, out)
			}
		}
		return nil
	})
}

func (w *world) expectReloaded() error {
	return w.eventually(func() error {
		calls, err := w.hledgerCalls()
		if err != nil {
			return err
		}
		prints := 0
		for _, call := range calls {
			if slices.Contains(call, "print") && slices.Contains(call, "-O") && !slices.ContainsFunc(call, func(arg string) bool {
				return strings.HasPrefix(arg, "--forecast") || arg == "-"
			}) && slices.Contains(call, w.journal) {
				prints++
			}
		}
		if prints < 2 {
			return errors.New("the journal was not read again")
		}
		return nil
	})
}

func (w *world) expectTabCountStill(count string) error { return w.expectTabCount(count) }

func (w *world) expectUpcomingLists(description, date string) error {
	return w.expectUpcoming(func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, description) && strings.Contains(row.text, date) {
				return nil
			}
		}
		return fmt.Errorf("Upcoming lacks %q on %s", description, date)
	})
}

// expectCheckingChanged looks for a forecast of the checking account that is not the one of the fixture: its
// balance in three months, 17.761,00 EUR, is different once a new rule takes money out.
func (w *world) expectCheckingChanged() error {
	return w.onMilestoneRow("assets:checking", func(text string, _ int) error {
		if strings.Contains(text, "17.761,00 EUR") {
			return errors.New("the checking line is as it was")
		}
		return nil
	})
}

func registerScheduledFormSteps(sc *godog.ScenarioContext, w *world) {
	registerFormFieldSteps(sc, w)
	registerFormPickerSteps(sc, w)
	registerFormInputSteps(sc, w)
	registerFormWriteSteps(sc, w)
	registerFormResultSteps(sc, w)
}

func registerFormFieldSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see a form with the fields$`, func(table *godog.Table) error {
		for _, row := range table.Rows {
			if err := w.expectField(strings.TrimSpace(row.Cells[0].Value), true); err != nil {
				return err
			}
		}
		return nil
	})
	step(sc, `^the frequency choices are ((?:"[^"]*"\s*)+)$`, w.expectFrequencyChoices)
	step(sc, `^I do not see the field "([^"]*)"$`, func(label string) error { return w.expectField(label, false) })
	step(sc, `^I see the field "([^"]*)"$`, func(label string) error { return w.expectField(label, true) })
	step(sc, `^the focus is on "([^"]*)"$`, w.expectFocusOn)
	step(sc, `^I focus the field "([^"]*)"$`, w.focusField)
	step(sc, `^the field "([^"]*)" contains "([^"]*)"$`, w.expectFieldHolds)
	step(sc, `^the field "([^"]*)" is "([^"]*)"$`, w.expectFieldIs)
	step(sc, `^the form is closed$`, func() error { return w.expectFormClosed(true) })
	step(sc, `^no form is shown$`, func() error { return w.expectFormClosed(true) })
	step(sc, `^the form stays open$`, func() error { return w.expectFormClosed(false) })
	step(sc, `^the currency choices are the currencies declared in the journal$`, w.expectCurrencyChoices)
	step(sc, `^the currency is "([^"]*)"$`, func(name string) error { return w.expectFieldIs("currency", name) })
	step(sc, `^the currency is the first declared currency$`, func() error {
		declared, err := declaredCommodities(w.journalFiles())
		if err != nil {
			return err
		}
		return w.expectFieldIs("currency", declared[0])
	})
}

func registerFormPickerSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the (from|to)-account picker does not offer "([^"]*)"$`, func(side, account string) error {
		return w.expectPickerOffers(side+" account", account, false)
	})
	step(sc, `^the account picker lists "([^"]*)"$`, func(account string) error { return w.expectFormPickerLists(account, true) })
	step(sc, `^the account picker does not list "([^"]*)"$`, func(account string) error {
		return w.expectFormPickerLists(account, false)
	})
}

// expectFormPickerLists waits for the picker under the focused account field to list the account, or not.
func (w *world) expectFormPickerLists(account string, listed bool) error {
	return w.onForm(func(rows []string) error {
		got := slices.ContainsFunc(rows, func(row string) bool {
			return strings.Contains(row, account) && !strings.Contains(row, "account  ")
		})
		if got != listed {
			return fmt.Errorf("the picker lists %q is %v, want %v: %q", account, got, listed, rows)
		}
		return nil
	})
}

func registerFormInputSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I choose the frequency "([^"]*)"$`, w.chooseFrequency)
	step(sc, `^I choose the from account "([^"]*)"$`, func(account string) error { return w.pickAccount("from account", account) })
	step(sc, `^I fill the form with valid values$`, func() error { return w.fillForm(validForm()) })
	step(sc, `^I fill the form with valid values except ([a-z ]+) is empty$`, func(field string) error {
		return w.fillExcept(field, "")
	})
	step(sc, `^I fill the form with valid values except ([a-z ]+) is "([^"]*)"$`, w.fillExcept)
	step(sc, `^I fill the form with the same account for from and to$`, func() error {
		values := validForm()
		values["to account"] = values["from account"]
		return w.fillForm(values)
	})
	step(sc, `^I confirm the form$`, func() error { return w.press("Ctrl+S") })
	step(sc, `^I see a message about the missing "([^"]*)"$`, w.expectFormMessage)
	step(sc, `^I type "([^"]*)" and press "([^"]*)"$`, func(text, key string) error {
		if err := w.typeText(text); err != nil {
			return err
		}
		return w.press(key)
	})
	step(sc, `^the field "to account" is empty or shows an error$`, func() error {
		return w.onForm(func(rows []string) error {
			value, _ := fieldRow(rows, "to account")
			if strings.TrimSpace(strings.ReplaceAll(value, "▏", "")) != "—" &&
				!slices.ContainsFunc(rows, func(row string) bool { return strings.Contains(row, "no account matches") }) {
				return fmt.Errorf("the to account holds %q", value)
			}
			return nil
		})
	})
	step(sc, `^I change the amount to "([^"]*)"$`, w.changeAmount)
}

func registerFormWriteSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, ruleDescription.String(), func(date, frequency, remaining, description, amount, currency, from, to string) error {
		return w.createRule(date, frequency, remaining, description, amount, currency, from, to)
	})
	step(sc, `^the journal contains "([^"]*)"$`, func(text string) error { return w.expectJournalHas(text, true) })
	step(sc, `^the journal does not contain "([^"]*)"$`, func(text string) error { return w.expectJournalHas(text, false) })
}

func registerFormResultSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the occurrences panel for "([^"]*)" lists exactly$`, w.expectOccurrencesFor)
	step(sc, `^"([^"]*)" contains "([^"]*)"$`, w.expectFileHas)
	step(sc, `^the main journal ends with the new rule$`, w.expectMainEndsWithRule)
	step(sc, `^the rule list shows "([^"]*)" with next date "([^"]*)"$`, w.expectRuleListRow)
	step(sc, `^the Upcoming panel lists "([^"]*)" on "([^"]*)"$`, w.expectUpcomingLists)
	step(sc, `^the checking line has changed$`, w.expectCheckingChanged)
	step(sc, `^the journal was reloaded$`, w.expectReloaded)
	step(sc, `^the tab bar still shows "(\d+)" transactions$`, w.expectTabCountStill)
	step(sc, `^hledger prints occurrences for "([^"]*)" on "([^"]*)" and "([^"]*)"$`, w.expectHledgerPrints)
	step(sc, `^the journal has the same number of lines$`, w.expectSameLineCount)
	step(sc, `^the block of "([^"]*)" (?:now )?shows "([^"]*)"$`, w.expectBlockShows)
	step(sc, `^the other four rules are unchanged$`, func() error { return w.expectOthersUnchanged("Acme GmbH") })
	step(sc, `^the other rules are byte-identical$`, func() error { return w.expectOthersUnchanged("Streaming", "Sparen") })
	step(sc, `^"([^"]*)" is gone$`, w.expectRuleGone)
	step(sc, `^the journal still contains "([^"]*)" directly above the rule$`, w.expectRuleDirectlyBelow)
	step(sc, `^I see a hint to edit the journal by hand$`, func() error {
		return w.expectLine(-1, "status bar", "edit the journal by hand", strings.Contains)
	})
}
