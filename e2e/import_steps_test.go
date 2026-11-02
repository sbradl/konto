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

const (
	pickerTitle  = "OPEN A CSV FILE"
	rowsTitle    = "ROWS"
	skippedTitle = "SKIPPED"
	infoTitle    = "IMPORT"
)

// openInImport picks the file in the download directory through the picker.
func (w *world) openInImport(name string) error {
	if err := w.press("2", "n"); err != nil {
		return err
	}
	var target int
	if err := w.expectPanel(pickerTitle, func(rows []recentRow) error {
		for i, row := range rows[1:] { // the first row is the directory
			if strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(row.text), "/")) == name {
				target = i
				return nil
			}
		}
		return fmt.Errorf("the picker does not list %q", name)
	}); err != nil {
		return err
	}
	for range target {
		if err := w.press("↓"); err != nil {
			return err
		}
	}
	if err := w.press("Enter"); err != nil {
		return err
	}
	s, err := w.session()
	if err != nil {
		return err
	}
	// opening reads the file asynchronously: what is typed next must not land in the picker
	s.waitFor(w.suite.timeout, func(screen string) bool { return !strings.Contains(screen, pickerTitle) })
	return nil
}

// importRow finds the row of the panel by description, and the date when given.
func (w *world) importRow(title, description, date string) (recentRow, error) {
	rows, err := w.tableRows(title)
	if err != nil {
		return recentRow{}, err
	}
	index, err := importRowIndex(rows, description, date)
	if err != nil {
		return recentRow{}, fmt.Errorf("%w in %s", err, title)
	}
	return rows[index], nil
}

func (w *world) expectImportRowHolds(description, date string, texts ...string) error {
	return w.eventually(func() error {
		row, err := w.importRow(rowsTitle, description, date)
		if err != nil {
			return err
		}
		for _, text := range texts {
			if !strings.Contains(row.text, text) {
				return fmt.Errorf("row %q lacks %q", row.text, text)
			}
		}
		return nil
	})
}

func (w *world) expectImportRowColour(description, date, text string, colour int) error {
	if err := w.expectImportRowHolds(description, date, text); err != nil {
		return err
	}
	row, _ := w.importRow(rowsTitle, description, date)
	before, _, _ := strings.Cut(row.text, text)
	if got := w.s.cellColour(row.x+1+len([]rune(before)), row.y); got != colour {
		return fmt.Errorf("%q in row %q is coloured %d, want %d", text, row.text, got, colour)
	}
	return nil
}

func (w *world) expectPanelRowCount(title string, n int) error {
	return w.expectPanel(title, func(rows []recentRow) error {
		count := 0
		for _, row := range rows {
			if !strings.HasPrefix(strings.TrimSpace(row.text), "…") {
				count++
			}
		}
		if count != n {
			return fmt.Errorf("%s has %d rows, want %d", title, count, n)
		}
		return nil
	})
}

func (w *world) expectInfo(label, value string) error {
	return w.expectRowHolds(infoTitle, label, value)
}

func (w *world) expectRowState(description, amount, date, state string) error {
	title := rowsTitle
	if state == "skipped" {
		title = skippedTitle
	}
	return w.expectPanel(title, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, description) && strings.Contains(row.text, amount) && strings.Contains(row.text, date) {
				return nil
			}
		}
		return fmt.Errorf("%q %s %s is not among the %s rows", description, amount, date, state)
	})
}

// selectedRowIndex is the index of the highlighted row of the table, and whether its text is bright white.
func (w *world) selectedRowIndex(rows []recentRow) (int, error) {
	for i, row := range rows {
		if w.s.cellBackground(row.x+4, row.y) != selectionBackground {
			continue
		}
		if fg := w.s.cellColour(row.x+9, row.y); fg != 15 {
			return 0, fmt.Errorf("the selected row has foreground %d", fg)
		}
		return i, nil
	}
	return 0, errors.New("no row is selected")
}

func (w *world) expectSelectedRow(which string) error {
	return w.expectPanel(rowsTitle, func(rows []recentRow) error {
		got, err := w.selectedRowIndex(rows)
		if err != nil {
			return err
		}
		if want := map[string]int{"first": 0, "second": 1, "last": len(rows) - 1}[which]; got != want {
			return fmt.Errorf("row %d is selected, want the %s", got, which)
		}
		return nil
	})
}

func (w *world) expectPickerLists(names ...string) error {
	return w.expectPanel(pickerTitle, func(rows []recentRow) error {
		for _, name := range names {
			found := false
			for _, row := range rows {
				found = found || strings.Contains(row.text, name)
			}
			if !found {
				return fmt.Errorf("the picker lacks %q", name)
			}
		}
		return nil
	})
}

func (w *world) expectPickerLacks(names ...string) error {
	if err := w.expectPickerLists(".."); err != nil {
		return err
	}
	rows, _ := w.tableRows(pickerTitle)
	for _, name := range names {
		for _, row := range rows {
			if strings.Contains(row.text, name) {
				return fmt.Errorf("the picker lists %q", name)
			}
		}
	}
	return nil
}

func (w *world) expectSkippedRows(table *godog.Table) error {
	return w.expectPanel(skippedTitle, func(rows []recentRow) error {
		if len(rows) != len(table.Rows) {
			return fmt.Errorf("%d skipped rows, want %d", len(rows), len(table.Rows))
		}
		for i, want := range table.Rows {
			for _, cell := range want.Cells {
				if !strings.Contains(rows[i].text, strings.TrimSpace(cell.Value)) {
					return fmt.Errorf("skipped row %d is %q, want %q", i+1, rows[i].text, cell.Value)
				}
			}
		}
		return nil
	})
}

func (w *world) expectSkippedDim() error {
	return w.expectPanel(skippedTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if got := w.s.cellColour(row.x+3, row.y); got != 8 {
				return fmt.Errorf("skipped row %q is coloured %d, want 8", row.text, got)
			}
		}
		return nil
	})
}

func (w *world) expectNoStateFiles() error {
	entries, err := os.ReadDir(filepath.Dir(w.journal))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if name := entry.Name(); !entry.IsDir() && !allowedBesideJournal(name) {
			return fmt.Errorf("unexpected file %s next to the journal", name)
		}
	}
	return nil
}

// allowedBesideJournal are the files konto and the harness leave beside the journal; anything else, a
// .latest state file above all, means konto wrote something it should not.
func allowedBesideJournal(name string) bool {
	switch filepath.Ext(name) {
	case ".journal", ".lock", ".log", ".json":
		return true
	}
	return name == "categories.rules"
}

func (w *world) expectNothingStaged() error {
	return w.eventually(func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if strings.Contains(s.screenText(), "╭─ ROWS ") {
			return errors.New("an import is staged")
		}
		return nil
	})
}

func (w *world) expectNoStatusDots() error {
	return w.expectPanel(infoTitle, func(rows []recentRow) error {
		for _, row := range rows {
			if strings.Contains(row.text, "●") {
				return errors.New("the Import view shows status dots")
			}
		}
		return nil
	})
}

func (w *world) expectPickerOrMessage() error {
	return w.eventually(func() error {
		s, err := w.session()
		if err != nil {
			return err
		}
		if screen := s.screenText(); !strings.Contains(screen, "OPEN A CSV FILE") && !strings.Contains(screen, "no such file") {
			return errors.New("neither a picker nor a message")
		}
		return nil
	})
}

func registerImportPickerSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I open "([^"]*)" in Import$`, w.openInImport)
	step(sc, `^I see an empty Import with a hint to press "n" for a new file$`, func() error {
		return w.expectScreen(true, "press n for a new file")
	})
	step(sc, `^I see the empty Import$`, func() error { return w.expectScreen(true, "press n for a new file") })
	step(sc, `^I do not see a file list$`, func() error { return w.expectScreen(false, "OPEN A CSV FILE") })
	step(sc, `^I do not see status dots$`, w.expectNoStatusDots)
	step(sc, `^I see a file picker$`, func() error { return w.expectScreen(true, "OPEN A CSV FILE") })
	step(sc, `^the picker shows the download directory path$`, func() error { return w.expectScreen(true, w.downloads) })
	step(sc, `^the picker shows "~/Downloads"$`, func() error { return w.expectScreen(true, filepath.Join(w.home, "Downloads")) })
	step(sc, `^the picker lists "([^"]*)" and "([^"]*)"$`, func(a, b string) error { return w.expectPickerLists(a, b) })
	step(sc, `^the picker does not list "([^"]*)" or "([^"]*)"$`, func(a, b string) error { return w.expectPickerLacks(a, b) })
	step(sc, `^I enter "([^"]*)" in the picker$`, func(name string) error { return w.selectInPicker(name) })
	step(sc, `^I select "([^"]*)" with "Enter"$`, func(name string) error { return w.selectInPicker(name) })
	step(sc, `^I see a file picker or a message naming the missing directory$`, w.expectPickerOrMessage)
}

// selectInPicker moves to the entry in the open picker and presses Enter.
func (w *world) selectInPicker(name string) error {
	var target int
	if err := w.expectPanel(pickerTitle, func(rows []recentRow) error {
		for i, row := range rows[1:] {
			if strings.Contains(row.text, name) {
				target = i
				return nil
			}
		}
		return fmt.Errorf("the picker does not list %q", name)
	}); err != nil {
		return err
	}
	for range target {
		if err := w.press("↓"); err != nil {
			return err
		}
	}
	return w.press("Enter")
}

func registerImportRowSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see the row table$`, func() error { return w.expectScreen(true, "╭─ ROWS ") })
	step(sc, `^the row table has (?:exactly )?(\d+) rows?$`, func(n int) error { return w.expectPanelRowCount(rowsTitle, n) })
	step(sc, `^the row table has no rows$`, func() error { return w.expectPanelRowCount(rowsTitle, 0) })
	step(sc, `^the row table has no row "([^"]*)"$`, func(name string) error {
		return w.expectPanel(rowsTitle, func(rows []recentRow) error {
			if _, err := rowFor(rows, name); err == nil {
				return fmt.Errorf("the row table has %q", name)
			}
			return nil
		})
	})
	step(sc, `^the row "([^"]*)" on "([^"]*)" shows "([^"]*)" coloured (\d+) and rule id "([^"]*)"$`,
		func(name, date, mark string, colour int, id string) error {
			if err := w.expectImportRowColour(name, date, mark, colour); err != nil {
				return err
			}
			return w.expectImportRowHolds(name, date, "#"+id)
		})
	step(sc, `^the row "([^"]*)" on "([^"]*)" shows "([^"]*)" coloured (\d+)$`, func(name, date, mark string, colour int) error {
		return w.expectImportRowColour(name, date, mark, colour)
	})
	step(sc, `^the row "([^"]*)" shows target "([^"]*)" coloured (\d+)$`, func(name, target string, colour int) error {
		return w.expectImportRowColour(name, "", target, colour)
	})
	step(sc, `^the table row "([^"]*)" shows "([^"]*)"$`, func(name, text string) error { return w.expectImportRowHolds(name, "", text) })
	step(sc, `^the row "([^"]*)" shows no rule id$`, func(name string) error {
		return w.eventually(func() error {
			row, err := w.importRow(rowsTitle, name, "")
			if err == nil && strings.Contains(row.text, "#") {
				return fmt.Errorf("row %q shows a rule id", row.text)
			}
			return err
		})
	})
	step(sc, `^the (first|second|last) table row is selected(?: with background \d+ and foreground \d+)?$`, w.expectSelectedRow)
}

func registerImportInfoSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the info block shows file "([^"]*)"$`, func(v string) error { return w.expectInfo("file", v) })
	step(sc, `^the info block shows account "([^"]*)"$`, func(v string) error { return w.expectInfo("account", v) })
	step(sc, `^the info block shows format "([^"]*)"$`, func(v string) error { return w.expectInfo("format", v) })
	step(sc, `^the info block shows watermark "([^"]*)"$`, func(v string) error { return w.expectInfo("watermark", v) })
	step(sc, `^the info block shows no watermark$`, func() error { return w.expectInfo("watermark", "no watermark") })
	step(sc, `^I do not see a list of existing accounts to attach to$`, func() error { return w.expectScreen(false, "attach") })
	step(sc, `^nothing is staged$`, w.expectNothingStaged)
	step(sc, `^I see a message saying the file is not a recognised export$`, func() error {
		return w.expectScreen(true, "not a recognised bank export")
	})
	step(sc, `^I see a message saying the account could not be detected$`, func() error {
		return w.expectScreen(true, "could not be detected")
	})
	step(sc, `^I see a message about the unreadable row or file$`, func() error { return w.expectScreen(true, "unreadable row or file") })
	step(sc, `^I see hledger's error text$`, func() error { return w.expectScreen(true, "hledger converting") })
	step(sc, `^the import is refused with a message naming "([^"]*)"$`, func(name string) error { return w.expectScreen(true, name) })
	step(sc, `^no file other than the journal and categories.rules exists next to the journal$`, w.expectNoStateFiles)
	step(sc, `^no "\.latest" file exists next to the journal or the CSV$`, w.expectNoStateFiles)
}

func registerImportSkippedSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^I see a "SKIPPED" list with exactly these rows$`, w.expectSkippedRows)
	step(sc, `^the skipped rows are coloured 8$`, w.expectSkippedDim)
	step(sc, `^the skipped panel shows (\d+) rows, not just the number \d+$`, func(n int) error { return w.expectPanelRowCount(skippedTitle, n) })
	step(sc, `^the skipped list has (\d+) rows?$`, func(n int) error { return w.expectPanelRowCount(skippedTitle, n) })
	step(sc, `^the skipped panel is empty$`, func() error { return w.expectPanelRowCount(skippedTitle, 0) })
	step(sc, `^the rows dated before "([^"]*)" are skipped$`, func(_ string) error {
		return w.expectPanel(skippedTitle, func(rows []recentRow) error {
			if len(rows) == 0 {
				return errors.New("nothing is skipped")
			}
			return nil
		})
	})
	step(sc, `^the rows dated after "([^"]*)" are new$`, func(_ string) error {
		return w.expectPanel(rowsTitle, func(rows []recentRow) error {
			if len(rows) == 0 {
				return errors.New("nothing is new")
			}
			return nil
		})
	})
	step(sc, `^"([^"]*)" of ([-\d.,]+) on "([^"]*)" is (new|skipped)$`, func(name, amount, date, state string) error {
		return w.expectRowState(name, amount, date, state)
	})
	step(sc, `^the row "([^"]*)" of ([-\d.,]+) is (new|skipped)$`, func(name, amount, state string) error {
		return w.expectRowState(name, amount, "", state)
	})
	step(sc, `^the row "([^"]*)" on "([^"]*)" is skipped$`, func(name, date string) error {
		return w.expectRowState(name, "", date, "skipped")
	})
	step(sc, `^the row "([^"]*)" is new$`, func(name string) error { return w.expectRowState(name, "", "", "new") })
	step(sc, `^"([^"]*)" is skipped$`, func(name string) error { return w.expectRowState(name, "", "", "skipped") })
}

// appendToJournal adds text to the end of the main journal.
func (w *world) appendToJournal(text string) error {
	f, err := os.OpenFile(w.journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.WriteString(text)
	return errors.Join(werr, f.Close())
}

// editDirective rewrites the `account` directive line of the account.
func (w *world) editDirective(account string, edit func(line string) string) error {
	b, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "account "+account+" ") || line == "account "+account {
			lines[i] = edit(line)
		}
	}
	return os.WriteFile(w.journal, []byte(strings.Join(lines, "\n")), 0o644)
}

func (w *world) laterTransaction(account, date string) error {
	return w.appendToJournal("\n" + date + " Later\n    " + account + "  1,00 EUR\n    equity:opening-balances\n")
}

func (w *world) extraChecking(date, description, category, amount string) error {
	return w.appendToJournal("\n" + date + " " + description + "\n    " + category + "  " + amount + "\n    assets:checking\n")
}

func (w *world) downloadDirContains(table *godog.Table) error {
	for _, row := range table.Rows {
		name := strings.TrimSpace(row.Cells[0].Value)
		path := filepath.Join(w.downloads, strings.TrimSuffix(name, "/"))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue // a file the scenario already made keeps its content
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

var sourceTag = regexp.MustCompile(`konto:source:[a-z]+(, )?`)

func registerImportFixtureSteps(sc *godog.ScenarioContext, w *world) {
	step(sc, `^the XDG download directory contains$`, w.downloadDirContains)
	step(sc, `^the XDG download directory does not exist$`, func() error { return os.RemoveAll(w.downloads) })
	step(sc, `^hledger fails on CSV conversion$`, func() error {
		w.shim.FailCSV = "hledger: simulated CSV conversion failure"
		return w.writeShim()
	})
	step(sc, `^a file "([^"]*)" in the download directory with random binary content$`, func(name string) error {
		return os.WriteFile(filepath.Join(w.downloads, name), []byte("\x00\x9f\x92\x96\xff\xfe\x01\x02\x80"), 0o644)
	})
	step(sc, `^a Postbank CSV without an IBAN line named "([^"]*)"$`, func(name string) error {
		b, err := postbankExport{IBAN: "", ClosingBalance: "1,00", Rows: []postbankRow{
			{Date: "29.9.2026", Type: "Gutschrift", Payee: "X", Purpose: "x", Amount: "1,00"},
		}}.render()
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(w.downloads, name), b, 0o644)
	})
	step(sc, `^the journal "([^"]*)" where IBAN ([A-Z0-9]+) is a closed account$`, func(name, _ string) error {
		return w.useJournal(name, "")
	})
	step(sc, `^the journal "([^"]*)" where "([^"]*)" is manual with IBAN ([A-Z0-9]+)$`, func(name, account, iban string) error {
		return w.useJournal(name, "\naccount "+account+"  ; konto:source:manual, konto:iban:"+iban+"\n")
	})
	step(sc, `^the journal "([^"]*)" where "([^"]*)" has no source tag$`, func(name, account string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.editDirective(account, func(line string) string {
			return sourceTag.ReplaceAllString(line, "")
		})
	})
	step(sc, `^the journal "([^"]*)" where the clearing account carries IBAN ([A-Z0-9]+)$`, func(name, iban string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.editDirective("assets:transfers", func(line string) string { return line + ", konto:iban:" + iban })
	})
	step(sc, `^the journal "([^"]*)" with a later (checking|savings) transaction on ([\d-]+)$`, func(name, which, date string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.laterTransaction("assets:"+which, date)
	})
	step(sc, `^the journal "([^"]*)" with an extra checking transaction on ([\d-]+): description "([^"]*)", category "([^"]*)", amount ([\d,.]+ EUR)$`,
		func(name, date, description, category, amount string) error {
			if err := w.useJournal(name, ""); err != nil {
				return err
			}
			return w.extraChecking(date, description, category, amount)
		})
}

func registerImportSteps(sc *godog.ScenarioContext, w *world) {
	registerImportPickerSteps(sc, w)
	registerImportRowSteps(sc, w)
	registerImportInfoSteps(sc, w)
	registerImportSkippedSteps(sc, w)
	registerImportFixtureSteps(sc, w)
}
