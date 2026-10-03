package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// rulesFile is the categorisation rules file beside the journal.
const rulesFile = "categories.rules"

func (f File) rulesPath() string { return filepath.Join(filepath.Dir(f.path), rulesFile) }

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
			if declares(line, edit.Directive, edit.Name) {
				return
			}
		}
	}
	line := edit.Directive + " " + edit.Name
	if edit.Comment != "" {
		line += "  ; " + edit.Comment
	}
	appendLine(sources[0], line)
}

// declares reports a line that is the directive naming exactly name, whatever follows the name.
func declares(line, directive, name string) bool {
	rest, ok := strings.CutPrefix(line, directive+" ")
	if !ok {
		return false
	}
	head, _, _ := strings.Cut(rest, ";")
	declared, _, _ := strings.Cut(strings.TrimSpace(head), "  ")
	return strings.TrimSpace(declared) == name
}

// addTransactions writes each transaction into the file at its date position.
func addTransactions(sources []*source, edit domain.AddTransactions) error {
	target := sources[0]
	if edit.File != "" {
		i := slices.IndexFunc(sources, func(s *source) bool { return samePath(s.path, edit.File) })
		if i < 0 {
			return fmt.Errorf("%s is not part of the journal", edit.File)
		}
		target = sources[i]
	}
	for _, transaction := range edit.Transactions {
		insertTransaction(target, transaction)
	}
	return nil
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
			for i > 0 && strings.HasPrefix(lines[i-1], ";") {
				i--
			}
			return i
		}
	}
	return -1
}

// headerDate is the date a transaction's first line starts with.
func headerDate(line string) (time.Time, bool) {
	if len(line) < len(time.DateOnly) {
		return time.Time{}, false
	}
	date, err := time.Parse(time.DateOnly, line[:len(time.DateOnly)])
	return date, err == nil
}

// setCategoryRule replaces the rule block with the id, or adds the block at the end of categories.rules.
func (w *workspace) setCategoryRule(edit domain.SetCategoryRule) error {
	if w.rules == nil {
		text, err := os.ReadFile(w.rulesPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reading %s: %w", w.rulesPath, err)
		}
		w.rules = &source{path: w.rulesPath, lines: strings.Split(string(text), "\n")}
	}
	block := strings.Split(edit.Text, "\n")
	marker := "comment2 konto-rule:" + edit.ID
	at := slices.IndexFunc(w.rules.lines, func(line string) bool { return strings.TrimSpace(line) == marker })
	if at < 0 {
		appendBlock(w.rules, block)
		return nil
	}
	first, last := at, at
	for first > 0 && w.rules.lines[first-1] != "" {
		first--
	}
	for last+1 < len(w.rules.lines) && w.rules.lines[last+1] != "" {
		last++
	}
	w.rules.lines = slices.Replace(w.rules.lines, first, last+1, block...)
	w.rules.dirty = true
	return nil
}

// samePath reports two spellings of one file: hledger reports the path as it opened it.
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
}
