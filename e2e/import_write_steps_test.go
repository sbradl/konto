//go:build e2e

package e2e

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cucumber/godog"
)

// importRow is a row of the Import table as the screen shows it.
var importRowText = regexp.MustCompile(`^\s*(\?|✓)\s+(?:#\d+\s+)?(\d{4}-\d{2}-\d{2})\s{2}(.*?)\s{2,}(.*)$`)

// questionRows are the rows of the table still marked "?": the date and description, and whether money came in.
type questionRow struct {
	index       int // the row's place in the table
	description string
	inflow      bool
}

func (w *world) questionRows() ([]questionRow, error) {
	var found []questionRow
	err := w.expectPanel(rowsTitle, func(rows []recentRow) error {
		found = nil
		for i, row := range rows {
			match := importRowText.FindStringSubmatch(row.text)
			if len(match) == 0 || match[1] != "?" {
				continue
			}
			found = append(found, questionRow{index: i, description: match[3], inflow: !strings.Contains(match[4], "-")})
		}
		return nil
	})
	return found, err
}

// transfers are the payees whose rows are moves between the user's own accounts, and where to.
func transfers() map[string]string { return map[string]string{"Max Mustermann": "assets:checking"} }

// categoriseEveryRow gives each "?" row a category of its direction, once.
func (w *world) categoriseEveryRow() error {
	for range 30 {
		rows, err := w.questionRows()
		if err != nil || len(rows) == 0 {
			return err
		}
		row := rows[0]
		category := cmp.Or(transfers()[row.description], map[bool]string{true: "income:salary", false: "expenses:groceries"}[row.inflow])
		if err := w.categoriseRowAt(row.index, category); err != nil {
			return err
		}
	}
	return errors.New("rows keep needing a category")
}

func (w *world) categoriseOnce(description, date, category string) error {
	if err := w.selectRow(description, date); err != nil {
		return err
	}
	return w.chooseOnce(category)
}

// categoriseRowAt gives the row at the place in the table its category, once.
func (w *world) categoriseRowAt(index int, category string) error {
	if err := w.press(append([]string{"Home"}, repeat("↓", index)...)...); err != nil {
		return err
	}
	return w.chooseOnce(category)
}

func (w *world) chooseOnce(category string) error {
	if err := w.press("Enter"); err != nil {
		return err
	}
	if err := w.chooseOption("once"); err != nil {
		return err
	}
	return w.pickCategory(category)
}

// categoriseRows applies the "how" column of a table: a new rule (with a category picked, created or a
// transfer target) or a choice for the row alone.
func (w *world) categoriseRows(table *godog.Table) error {
	for _, row := range table.Rows[1:] {
		name, how := row.Cells[0].Value, row.Cells[1].Value
		kind, target, _ := strings.Cut(how, " → ")
		var err error
		switch {
		case kind == "once":
			err = w.categoriseOnce(name, "", target)
		case strings.HasPrefix(target, "create "):
			err = w.startNewRule(name, strings.TrimPrefix(target, "create "), true)
		default:
			err = w.startNewRule(name, strings.TrimPrefix(target, "transfer to "), false)
		}
		if err == nil && kind != "once" {
			err = w.press("Enter")
		}
		if err != nil {
			return fmt.Errorf("categorising %q: %w", name, err)
		}
	}
	return nil
}

// writeImport asks to write and confirms; a write that is blocked has no question to answer. A confirmed write
// is waited for, and then for the views to reload, as the next import reads the journal as it is then.
func (w *world) writeImport() error {
	if err := w.press("w"); err != nil {
		return err
	}
	if line, err := w.statusLine(); err != nil || !strings.Contains(line, "(y/n)") {
		return err
	}
	if err := w.press("y"); err != nil {
		return err
	}
	if err := w.expectLine(-1, "status bar", "imported ", strings.Contains); err != nil {
		return err
	}
	return w.eventually(func() error {
		line, err := w.statusLine()
		if err == nil && strings.Contains(line, "⣾") {
			err = errors.New("still reloading")
		}
		return err
	})
}

// openUnlessStaged opens the file in Import, unless that very file is already staged: what was staged for it
// then goes into the import.
func (w *world) openUnlessStaged(file string) error {
	s, err := w.session()
	if err != nil {
		return err
	}
	w.waitLoaded(s)
	s.settle(w.suite.timeout)
	if strings.Contains(s.screenText(), "│ file      "+file) {
		return nil
	}
	return w.openInImport(file)
}

func (w *world) importEveryRow(file string) error {
	if err := w.openUnlessStaged(file); err != nil {
		return err
	}
	if err := w.categoriseEveryRow(); err != nil {
		return err
	}
	return w.writeImport()
}

func (w *world) importCreatingCategory(file, category, payee string) error {
	if err := w.openUnlessStaged(file); err != nil {
		return err
	}
	if err := w.startNewRule(payee, category, true); err != nil {
		return err
	}
	if err := w.press("Enter"); err != nil {
		return err
	}
	if err := w.categoriseEveryRow(); err != nil {
		return err
	}
	return w.writeImport()
}

func (w *world) setAccountName(name string) error {
	if err := w.press("Ctrl+U"); err != nil {
		return err
	}
	if name != "" {
		if err := w.typeText(name); err != nil {
			return err
		}
	}
	return w.press("Enter")
}

// createAccount names the new account; from a file, it abandons what is open and opens that file first.
func (w *world) createAccount(name, file string) error {
	if file != "" {
		if err := w.press("Esc"); err != nil {
			return err
		}
		if err := w.openInImport(file); err != nil {
			return err
		}
	}
	return w.setAccountName(name)
}

func (w *world) createAndImport(name, file string) error {
	if err := w.createAccount(name, file); err != nil {
		return err
	}
	if err := w.categoriseEveryRow(); err != nil {
		return err
	}
	return w.writeImport()
}

// accountNameInput is what the account-name dialog holds.
func (w *world) accountNameInput() (string, error) {
	d, err := w.dialogNow()
	if err != nil {
		return "", err
	}
	if d.title != "create account" {
		return "", fmt.Errorf("the %q dialog is open", d.title)
	}
	for _, row := range d.rows {
		if text, ok := strings.CutPrefix(strings.TrimLeft(row, "▸ "), "name"); ok {
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "▏")), nil
		}
	}
	return "", fmt.Errorf("the dialog has no name input: %q", d.rows)
}

// expectWritePreview waits for the write preview to show the lines one after the other.
func (w *world) expectWritePreview(want []string) error {
	normalised := func(lines []string) []string {
		out := make([]string, len(lines))
		for i, line := range lines {
			out[i] = normaliseLine(line)
		}
		return out
	}
	wanted := normalised(want)
	return w.withDialog(func(d dialog) error {
		if d.title != "write" {
			return fmt.Errorf("the %q dialog is open, want the write preview", d.title)
		}
		if !containsSeq(normalised(d.rows), wanted) {
			return fmt.Errorf("the preview %q does not show %q", d.rows, wanted)
		}
		return nil
	})
}

// expectNoRowSkipped presses every key that might skip a row and checks the table did not shrink.
func (w *world) expectNoRowSkipped() error {
	count := func() (int, error) {
		n := 0
		err := w.expectPanel(rowsTitle, func(rows []recentRow) error { n = len(rows); return nil })
		return n, err
	}
	before, err := count()
	if err != nil {
		return err
	}
	for _, key := range []string{"s", "d", "x", "Delete", "Backspace", "Esc"} {
		if err := w.press(key); err != nil {
			return err
		}
	}
	after, err := count()
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("the table had %d rows and now has %d", before, after)
	}
	return nil
}

func (w *world) expectEveryRowCategorised() error {
	rows, err := w.questionRows()
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return fmt.Errorf("%d rows still need a category", len(rows))
	}
	return w.expectScreen(true, "╭─ ROWS ")
}

// expectScreenLineWith waits for one screen line to show every text.
func (w *world) expectScreenLineWith(texts ...string) error {
	return w.expectScreenLines(func(lines []string) error {
		for _, line := range lines {
			if !slices.ContainsFunc(texts, func(text string) bool { return !strings.Contains(line, text) }) {
				return nil
			}
		}
		return fmt.Errorf("no screen line shows all of %q", texts)
	})
}

// block is a transaction as written in a journal file: the dated line and the indented lines under it.
func blocksOf(text string) []string {
	var blocks []string
	var current []string
	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, strings.Join(current, "\n"))
		}
		current = nil
	}
	for line := range strings.SplitSeq(text, "\n") {
		switch {
		case dateLine.MatchString(line):
			flush()
			current = append(make([]string, 0, 3), line)
		case current != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")):
			current = append(current, line)
		default:
			flush()
		}
	}
	flush()
	return blocks
}

var dateLine = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

// journalBlocks are the transactions of every journal file now, optionally only the main journal.
func (w *world) journalBlocks(mainOnly bool) []string {
	var blocks []string
	for path, content := range w.journalFiles() {
		if !mainOnly || path == w.journal {
			blocks = append(blocks, blocksOf(string(content))...)
		}
	}
	return blocks
}

// newBlocks are the transactions that were not in the journal at start.
func (w *world) newBlocks(mainOnly bool) []string {
	var before []string
	for path, content := range w.filesAtStart {
		if !mainOnly || path == w.journal {
			before = append(before, blocksOf(string(content))...)
		}
	}
	var added []string
	for _, block := range w.journalBlocks(mainOnly) {
		if i := slices.Index(before, block); i >= 0 {
			before = slices.Delete(before, i, i+1)
			continue
		}
		added = append(added, block)
	}
	return added
}

// blockStarting finds the transaction whose first line starts with the text.
func blockStarting(blocks []string, header string) (string, error) {
	for _, block := range blocks {
		if strings.HasPrefix(block, header) {
			return block, nil
		}
	}
	return "", fmt.Errorf("no transaction starts %q", header)
}

func (w *world) expectNewBlock(header string, check func(block string) error) error {
	return w.eventually(func() error {
		block, err := blockStarting(w.newBlocks(false), header)
		if err != nil {
			return err
		}
		return check(block)
	})
}

var postingAssertion = regexp.MustCompile(`(?m)^\s+\S.*\s=\s*-?[\d.,]+`)

func (w *world) expectAssertion(header, assertion, account string) error {
	return w.expectNewBlock(header, func(block string) error {
		for line := range strings.SplitSeq(block, "\n") {
			if strings.Contains(line, account) && strings.Contains(line, assertion) {
				return nil
			}
		}
		return fmt.Errorf("%q has no %q on %s", block, assertion, account)
	})
}

func (w *world) expectNoOtherAssertion() error {
	return w.eventually(func() error {
		for _, block := range w.newBlocks(false) {
			if !strings.HasPrefix(block, "2026-09-30 REWE Markt") && postingAssertion.MatchString(block) {
				return fmt.Errorf("an imported transaction has an assertion: %q", block)
			}
		}
		return nil
	})
}

func (w *world) expectNoSecondaryDate() error {
	secondary := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}=`)
	return w.eventually(func() error {
		blocks := w.newBlocks(false)
		if len(blocks) == 0 {
			return errors.New("nothing was imported")
		}
		for _, block := range blocks {
			if secondary.MatchString(block) {
				return fmt.Errorf("a secondary date: %q", block)
			}
		}
		return nil
	})
}

func (w *world) expectNoBankFields() error {
	return w.eventually(func() error {
		blocks := w.newBlocks(false)
		if len(blocks) == 0 {
			return errors.New("nothing was imported")
		}
		for _, block := range blocks {
			for _, banned := range []string{"DE89", "DE27", "BIC", "Kartenzahlung", "Lastschrift", "Gutschrift", "Dauerauftrag", "Umbuchung"} {
				if strings.Contains(block, banned) {
					return fmt.Errorf("%q contains %q", block, banned)
				}
			}
		}
		return nil
	})
}

func (w *world) expectCountOnDate(want int, description, checking, date string) error {
	return w.eventually(func() error {
		count := 0
		for _, block := range w.journalBlocks(false) {
			if headedBy(block, date, description) && (checking == "" || strings.Contains(block, "assets:checking")) {
				count++
			}
		}
		if count != want {
			return fmt.Errorf("%d %q transactions on %s, want %d", count, description, date, want)
		}
		return nil
	})
}

// headedBy reports a transaction dated date whose description is exactly the text, comment aside.
func headedBy(block, date, description string) bool {
	header, _, _ := strings.Cut(block, "\n")
	header, _, _ = strings.Cut(header, "  ;")
	return header == date+" "+description
}

// postsWith reports a posting line of the block that holds every text.
func postsWith(block string, texts ...string) bool {
	for line := range strings.SplitSeq(block, "\n") {
		if !slices.ContainsFunc(texts, func(text string) bool { return !strings.Contains(line, text) }) {
			return true
		}
	}
	return false
}

// declaredLines counts the lines of every journal file that declare the directive text.
func (w *world) declaredLines(text string) int {
	count := 0
	for _, content := range w.journalFiles() {
		for line := range strings.SplitSeq(string(content), "\n") {
			if line == text || strings.HasPrefix(line, text+"  ") {
				count++
			}
		}
	}
	return count
}

func (w *world) expectDeclares(text string, times int) error {
	return w.eventually(func() error {
		if got := w.declaredLines(text); got != times {
			return fmt.Errorf("%q is declared %d times, want %d", text, got, times)
		}
		return nil
	})
}

func (w *world) expectBalance(account, want string) error {
	return w.eventually(func() error {
		out, err := exec.CommandContext(context.Background(), w.suite.realHledger,
			"-f", w.journal, "bal", account, "-N", "--no-total").CombinedOutput()
		if err != nil {
			return fmt.Errorf("hledger bal: %w: %s", err, out)
		}
		if !strings.Contains(string(out), want) {
			return fmt.Errorf("the balance of %s is %q, want %s", account, strings.TrimSpace(string(out)), want)
		}
		return nil
	})
}

func (w *world) expectCheck(name string) error {
	return w.eventually(func() error {
		out, err := exec.CommandContext(context.Background(), w.suite.realHledger, "-f", w.journal, "check", name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("check %s: %s", name, out)
		}
		return nil
	})
}

func (w *world) expectOpeningBalance(date, amount, account, assertion string) error {
	return w.eventually(func() error {
		for _, block := range w.journalBlocks(false) {
			if headedBy(block, date, "opening balance") && postsWith(block, account, amount, assertion) {
				return nil
			}
		}
		return fmt.Errorf("no opening balance on %s posts %s to %s with %s", date, amount, account, assertion)
	})
}

func (w *world) expectOpeningCounter(date, account string) error {
	return w.eventually(func() error {
		for _, block := range w.journalBlocks(false) {
			if strings.HasPrefix(block, date+" opening balance") && strings.Contains(block, account) {
				return nil
			}
		}
		return fmt.Errorf("no opening balance on %s posts to %q", date, account)
	})
}

// splitJournal moves the transactions that match into a file the main journal includes.
func (w *world) splitJournal(relative string, moves func(block string) bool) error {
	text, err := os.ReadFile(w.journal)
	if err != nil {
		return err
	}
	blocks := blocksOf(string(text))
	head, _, _ := strings.Cut(string(text), blocks[0])
	var moved, kept []string
	for _, block := range blocks {
		if moves(block) {
			moved = append(moved, block)
		} else {
			kept = append(kept, block)
		}
	}
	path := filepath.Join(w.dir, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(strings.Join(moved, "\n\n")+"\n"), 0o644); err != nil {
		return err
	}
	main := head + "include " + relative + "\n\n" + strings.Join(kept, "\n\n") + "\n"
	return os.WriteFile(w.journal, []byte(main), 0o644)
}

func (w *world) expectInFile(relative, header string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(filepath.Join(w.dir, relative))
		if err != nil {
			return err
		}
		if _, err := blockStarting(blocksOf(string(content)), header); err != nil {
			return fmt.Errorf("%s: %w", relative, err)
		}
		return nil
	})
}

func (w *world) expectMainHasNoNewTransaction() error {
	return w.eventually(func() error {
		if added := w.newBlocks(true); len(added) > 0 {
			return fmt.Errorf("the main journal got %q", added)
		}
		return nil
	})
}

func (w *world) expectMainContains(text string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(w.journal)
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), text) {
			return fmt.Errorf("the main journal lacks %q", text)
		}
		return nil
	})
}

func (w *world) expectLastTransactionDated(date string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(w.journal)
		if err != nil {
			return err
		}
		blocks := blocksOf(string(content))
		if len(blocks) == 0 || !strings.HasPrefix(blocks[len(blocks)-1], date) {
			return fmt.Errorf("the last transaction is not dated %s", date)
		}
		return nil
	})
}

func (w *world) expectAbove(upper, lower string) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(w.journal)
		if err != nil {
			return err
		}
		text := string(content)
		first, second := strings.Index(text, upper), strings.Index(text, lower)
		if first < 0 || second < 0 || first > second {
			return fmt.Errorf("%q is not above %q", upper, lower)
		}
		return nil
	})
}

func (w *world) expectRulesMention(text string, want bool) error {
	return w.eventually(func() error {
		content, err := os.ReadFile(w.rules)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), text) != want {
			return fmt.Errorf("categories.rules mention %q is %v, want %v", text, !want, want)
		}
		return nil
	})
}

func (w *world) expectRulesKeepTheOldOnes() error {
	return w.eventually(func() error {
		content, err := os.ReadFile(w.rules)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(content), strings.TrimRight(string(w.rulesAtStart), "\n")) {
			return fmt.Errorf("categories.rules no longer starts with its old rules:\n%s", content)
		}
		return nil
	})
}

func (w *world) expectNoTempFiles() error {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".konto-tmp") {
			return fmt.Errorf("left behind: %s", entry.Name())
		}
	}
	return nil
}

func registerImportWriteSteps(sc *godog.ScenarioContext, w *world) {
	registerWriteFlowSteps(sc, w)
	registerWriteJournalSteps(sc, w)
	registerWriteFixtureSteps(sc, w)
	registerWriteAccountSteps(sc, w)
}

func registerWriteFlowSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^I categorise the rows$`, w.categoriseRows)
	sc.Step(`^I open "([^"]*)" in Import and categorise every row$`, func(file string) error {
		if err := w.openInImport(file); err != nil {
			return err
		}
		return w.categoriseEveryRow()
	})
	sc.Step(`^I write the import$`, w.writeImport)
	sc.Step(`^I import "([^"]*)" categorising every row$`, w.importEveryRow)
	sc.Step(`^I import "([^"]*)" creating the category "([^"]*)" for "([^"]*)"$`, w.importCreatingCategory)
	sc.Step(`^no key skips or dismisses a "\?" row$`, w.expectNoRowSkipped)
	sc.Step(`^I do not see "skip" in the key hints$`, func() error {
		return w.expectLine(-1, "status bar", "skip", func(line, text string) bool { return !strings.Contains(line, text) })
	})
	sc.Step(`^I see a y/n prompt$`, func() error { return w.expectStatusHas(true, []string{"(y/n)"}) })
	sc.Step(`^I see a preview containing$`, func(doc *godog.DocString) error {
		return w.expectWritePreview(strings.Split(strings.TrimSpace(doc.Content), "\n"))
	})
	sc.Step(`^I see a preview containing "([^"]*)"$`, func(text string) error {
		return w.withDialog(func(d dialog) error {
			if d.title != "write" || !d.holds(text) {
				return fmt.Errorf("the %q dialog does not show %q: %q", d.title, text, d.rows)
			}
			return nil
		})
	})
	sc.Step(`^the row table is still shown with every row categorised$`, w.expectEveryRowCategorised)
	sc.Step(`^the row table is still shown$`, func() error { return w.expectScreen(true, "╭─ ROWS ") })
	sc.Step(`^the write is blocked$`, func() error { return w.expectScreen(true, "write blocked") })
	sc.Step(`^I see "([^"]*)" "([^"]*)"$`, func(label, value string) error { return w.expectScreenLineWith(label, value) })
	sc.Step(`^the journal file was never modified during the attempt$`, w.expectJournalFilesUnchanged)
	sc.Step(`^the write succeeds$`, func() error { return w.expectScreen(true, "imported ") })
}

func registerWriteJournalSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal contains no "([^"]*)" or "([^"]*)" account$`, func(a, b string) error {
		return w.eventually(func() error {
			text := strings.ToLower(allJournalText(w.journalFiles()))
			if len(w.newBlocks(false)) == 0 || strings.Contains(text, a) || strings.Contains(text, b) {
				return fmt.Errorf("nothing was written, or the journal mentions %q or %q", a, b)
			}
			return nil
		})
	})
	sc.Step(`^the journal's "([^"]*)" transaction on "([^"]*)" has no "([^"]*)" tag$`, func(description, date, tag string) error {
		return w.expectNewBlock(date+" "+description, func(block string) error {
			if strings.Contains(block, tag) {
				return fmt.Errorf("%q has a %s tag", block, tag)
			}
			return nil
		})
	})
	sc.Step(`^the "([^"]*)" transaction has the assertion "([^"]*)" on "([^"]*)"$`, w.expectAssertion)
	sc.Step(`^no other imported transaction has an assertion$`, w.expectNoOtherAssertion)
	sc.Step(`^no imported transaction has a secondary date$`, w.expectNoSecondaryDate)
	sc.Step(`^the imported transactions contain no IBAN, BIC, reference or transaction type$`, w.expectNoBankFields)
	sc.Step(`^the journal has exactly (\d+) "([^"]*)"( checking)? transaction on "([^"]*)"$`, func(n int, description, checking, date string) error {
		return w.expectCountOnDate(n, description, strings.TrimSpace(checking), date)
	})
	sc.Step(`^the journal balance of "([^"]*)" is ([\d.,-]+ EUR)$`, w.expectBalance)
	sc.Step(`^the journal declares "([^"]*)"$`, func(text string) error { return w.expectDeclares(text, 1) })
	sc.Step(`^"([^"]*)" is declared exactly once$`, func(text string) error { return w.expectDeclares(text, 1) })
	sc.Step(`^hledger check ([a-z]+) passes$`, w.expectCheck)
	sc.Step(`^categories.rules still contains rules 1 to 5 unchanged$`, w.expectRulesKeepTheOldOnes)
	sc.Step(`^categories.rules does not mention "([^"]*)"$`, func(text string) error { return w.expectRulesMention(text, false) })
	sc.Step(`^no "\.latest" file exists$`, w.expectNoStateFiles)
	sc.Step(`^no temporary rules file is left behind$`, w.expectNoTempFiles)
	sc.Step(`^no file other than the journal and categories.rules was written$`, w.expectNoStateFiles)
}

func registerWriteFixtureSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^the journal "([^"]*)" where every "([^"]*)" transaction lives in included file "([^"]*)"$`,
		func(name, account, file string) error {
			if err := w.useJournal(name, ""); err != nil {
				return err
			}
			return w.splitJournal(file, func(block string) bool { return strings.Contains(block, account) })
		})
	sc.Step(`^the journal "([^"]*)" where every transaction lives in included file "([^"]*)"$`, func(name, file string) error {
		if err := w.useJournal(name, ""); err != nil {
			return err
		}
		return w.splitJournal(file, func(string) bool { return true })
	})
	sc.Step(`^the journal "([^"]*)" with an unrelated transaction dated ([\d-]+) as the last entry of the file$`,
		func(name, date string) error {
			return w.useJournal(name, "\npayee Unrelated\n\n"+date+" Unrelated\n    assets:savings  1,00 EUR\n    equity:opening-balances\n")
		})
	sc.Step(`^the new transactions are in "([^"]*)"$`, func(file string) error { return w.expectInFile(file, "2026-09-30 REWE Markt") })
	sc.Step(`^the main journal has no new transaction$`, w.expectMainHasNoNewTransaction)
	sc.Step(`^the main journal contains "([^"]*)"$`, w.expectMainContains)
	sc.Step(`^the main journal contains the opening balance for "([^"]*)"$`, func(account string) error {
		return w.expectMainContains("opening balance\n    " + account)
	})
	sc.Step(`^the journal's "opening balance" on "([^"]*)" posts ([\d.,-]+ EUR) to "([^"]*)" with assertion "([^"]*)"$`,
		w.expectOpeningBalance)
	sc.Step(`^the journal's "opening balance" on "([^"]*)" posts to "([^"]*)"$`, w.expectOpeningCounter)
	sc.Step(`^the journal's "opening balance" is dated "([^"]*)"$`, func(date string) error {
		return w.expectOpeningCounter(date, "")
	})
	sc.Step(`^the "1\.9\." transfer posts against "([^"]*)"$`, func(account string) error {
		return w.expectNewBlock("2026-09-01 ", func(block string) error {
			if !strings.Contains(block, account) {
				return fmt.Errorf("%q does not post against %s", block, account)
			}
			return nil
		})
	})
	sc.Step(`^the last transaction of the file is dated "([^"]*)"$`, w.expectLastTransactionDated)
	sc.Step(`^the "([^"]*)" transaction is above the unrelated "([^"]*)" transaction$`, func(upper, date string) error {
		return w.expectAbove(upper, date+" Unrelated")
	})
}

func registerWriteAccountSteps(sc *godog.ScenarioContext, w *world) {
	sc.Step(`^I see the account name input$`, func() error { _, err := w.accountNameInput(); return err })
	sc.Step(`^the suggested name is not empty$`, func() error {
		name, err := w.accountNameInput()
		if err == nil && name == "" {
			err = errors.New("the suggested name is empty")
		}
		return err
	})
	sc.Step(`^the suggested name starts with "([^"]*)" or "([^"]*)"$`, func(a, b string) error {
		name, err := w.accountNameInput()
		if err == nil && !strings.HasPrefix(name, a) && !strings.HasPrefix(name, b) {
			err = fmt.Errorf("the suggested name %q starts with neither %q nor %q", name, a, b)
		}
		return err
	})
	sc.Step(`^I set the account name to "([^"]*)"$`, w.setAccountName)
	sc.Step(`^I see a name refusal mentioning "([^"]*)"$`, func(text string) error {
		return w.withDialog(func(d dialog) error {
			if !d.holds("refused:") || !d.holds(text) {
				return fmt.Errorf("no refusal mentions %q: %q", text, d.rows)
			}
			return nil
		})
	})
	sc.Step(`^I cannot continue$`, func() error { return w.expectDialogTitle("create account") })
	sc.Step(`^I create the account "([^"]*)"$`, func(name string) error { return w.createAccount(name, "") })
	sc.Step(`^I create the account "([^"]*)"(?: from "([^"]*)")? and import every row$`, w.createAndImport)
	sc.Step(`^I create the account "([^"]*)" from "([^"]*)"$`, w.createAccount)
	sc.Step(`^the row "([^"]*)" shows the glyph "([^"]*)"$`, func(account, glyph string) error {
		return w.expectAccounts(expectShows(account, glyph))
	})
}
