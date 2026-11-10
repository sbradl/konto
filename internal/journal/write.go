package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/rulesfile"
)

// RulesPath is the categorisation rules file beside the journal.
func (f File) RulesPath() string { return filepath.Join(filepath.Dir(f.path), "categories.rules") }

// appendLine adds a line at the end of the file, keeping its final newline last.
func appendLine(file *source, line string) {
	if last := len(file.lines) - 1; file.lines[last] == "" {
		file.lines = slices.Insert(file.lines, last, line)
	} else {
		file.lines = append(file.lines, line)
	}
	file.dirty = true
}

// appendBlock adds lines at the end of the file after a blank line, and ends the file with a newline.
func appendBlock(file *source, block []string) {
	lines := slices.Clone(file.lines)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}
	file.lines = append(append(lines, block...), "")
	file.dirty = true
}

// declare adds the directive to the main journal unless a file has it already.
func declare(sources []*source, edit domain.Declare) {
	for _, source := range sources {
		for _, line := range source.lines {
			if directive, ok := parseDirective(line, edit.Directive); ok && declaredName(directive, edit) == edit.Name {
				return
			}
		}
	}
	appendLine(sources[0], edit.Line())
}

// declaredName is the name a directive line declares: two spaces end an account name, but a payee or tag name
// runs to the comment.
func declaredName(directive directiveLine, edit domain.Declare) string {
	if edit.Directive == domain.DirectiveAccount {
		return directive.name
	}

	return directive.head
}

// addTransactions writes each transaction into the year file of its date, at its date position. A year
// file that is not there yet starts empty; a year after this one is refused.
func (w *workspace) addTransactions(edit domain.AddTransactions) error {
	for _, transaction := range edit.Transactions {
		target, err := w.yearFile(transaction.Date.Year())
		if err != nil {
			return err
		}
		insertTransaction(target, transaction)
		w.touched = append(w.touched, transaction.Date.Year())
	}

	return nil
}

// yearFile is the source of the year's file: read when it is on disk and not loaded yet, started in memory
// when the year has none.
func (w *workspace) yearFile(year int) (*source, error) {
	if year > w.thisYear {
		return nil, domain.RefusalError{Reason: fmt.Sprintf(
			"a transaction dated %d is after this year, %d", year, w.thisYear)}
	}

	if found := w.findYear(year); found != nil {
		return found, nil
	}

	if slices.Contains(w.onDisk, year) {
		return w.readYear(year)
	}
	created := &source{path: w.yearPath(year), lines: []string{""}}
	w.sources = append(w.sources, created)
	w.created[year] = true

	return created, nil
}

// addRule appends the `~` block to the main journal after a blank line.
func addRule(sources []*source, edit domain.AddRule) {
	appendBlock(sources[0], edit.Lines)
}

// insertTransaction puts the transaction above the first transaction dated after it, or at the end.
func insertTransaction(file *source, transaction domain.NewTransaction) {
	later := firstDatedAfter(file.lines, transaction.Date)
	if later < 0 {
		appendBlock(file, transaction.Lines())

		return
	}
	block := append(transaction.Lines(), "")
	file.lines = slices.Insert(file.lines, later, block...)
	file.dirty = true
}

// firstDatedAfter is the line where the first transaction dated after date starts, its comment lines
// above it included; -1 when there is none.
func firstDatedAfter(lines []string, date time.Time) int {
	for i, line := range lines {
		if when, ok := headerDate(line); ok && when.After(date) {
			for i > 0 && strings.IndexAny(lines[i-1], ";#*") == 0 {
				i--
			}

			return i
		}
	}

	return -1
}

// setCategoryRule replaces the rule block with the id, or adds the block at the end of categories.rules.
func (w *workspace) setCategoryRule(edit domain.SetCategoryRule) error {
	if err := w.loadRules(); err != nil {
		return err
	}

	block := strings.Split(edit.Text, "\n")

	first, end, found := rulesfile.Block(w.rules.lines, edit.ID)
	if !found && edit.First {
		w.rules.lines = slices.Concat(block, []string{""}, w.rules.lines)
		w.rules.dirty = true

		return nil
	}

	if !found {
		appendBlock(w.rules, block)

		return nil
	}
	w.rules.lines = slices.Replace(w.rules.lines, first, end, block...)
	w.rules.dirty = true

	return nil
}

// loadRules reads categories.rules once; a missing file is an empty one.
func (w *workspace) loadRules() error {
	if w.rules != nil {
		return nil
	}

	text, err := os.ReadFile(w.rulesPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", w.rulesPath, err)
	}
	w.rules = &source{path: w.rulesPath, lines: strings.Split(string(text), "\n")}

	return nil
}
