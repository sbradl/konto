package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// workspace is the journal's files as edits change them, and the categories.rules file beside the journal.
type workspace struct {
	sources             []*source
	rulesPath           string
	rules               *source // read when an edit first needs it
	firstYear, lastYear int     // the window's years
	yearPath            func(year int) string
	touched             []int        // the years transactions were added to
	created             map[int]bool // the years whose file the edits started
}

// plan makes the edits in memory; nothing is written.
func (f File) plan(changeset domain.Changeset) (*workspace, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	work := &workspace{
		sources: sources, rulesPath: f.rulesPath(),
		firstYear: f.firstYear(), lastYear: f.lastYear(), yearPath: f.yearPath, created: map[int]bool{},
	}
	for _, edit := range changeset.Edits {
		if err := work.apply(edit); err != nil {
			return nil, err
		}
	}
	return work, nil
}

// files are the sources the edits changed.
func (w *workspace) dirty() []*source {
	changed := slices.DeleteFunc(slices.Clone(w.sources), func(s *source) bool { return !s.dirty })
	if w.rules != nil && w.rules.dirty {
		changed = append(changed, w.rules)
	}
	return changed
}

// Apply writes a changeset into the journal's files and categories.rules. Each touched file is replaced in
// one go (temp file, then rename), so a failed write never leaves a half-edited file; lines the edits do
// not name stay byte for byte as they were.
func (f File) Apply(changeset domain.Changeset) error {
	work, err := f.plan(changeset)
	if err != nil {
		return err
	}
	for _, source := range work.dirty() {
		if err := writeSource(source); err != nil {
			return err
		}
	}
	return nil
}

// Render is the journal as it would read after the changeset, its included files spliced in where they are
// included, so hledger can check it from standard input without anything being written.
func (f File) Render(changeset domain.Changeset) (string, error) {
	work, err := f.plan(changeset)
	if err != nil {
		return "", err
	}
	byPath := make(map[string]*source, len(work.sources))
	for _, file := range work.sources {
		byPath[file.path] = file
	}
	var text strings.Builder
	text.WriteString(flatten(byPath, work.sources[0], map[string]bool{}))
	for year := f.firstYear(); year <= f.lastYear(); year++ {
		if file, ok := byPath[f.yearPath(year)]; ok {
			text.WriteString(strings.Join(file.lines, "\n"))
		}
	}
	return text.String(), nil
}

// flatten is the file's text with each `include` line replaced by the text of the files it names. Every
// included file was read with the journal, so byPath has them all.
func flatten(byPath map[string]*source, file *source, seen map[string]bool) string {
	seen[file.path] = true
	var lines []string
	for _, line := range file.lines {
		if !strings.HasPrefix(line, "include ") {
			lines = append(lines, line)
			continue
		}
		matches, _ := includedFiles(file.path, line) // the files were read before, so the pattern is good
		for _, match := range matches {
			if !seen[match] {
				lines = append(lines, flatten(byPath, byPath[match], seen))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// writeSource replaces the file, or creates it when it is not there yet.
func writeSource(file *source) error {
	content := strings.Join(file.lines, "\n")
	if _, err := os.Stat(file.path); errors.Is(err, fs.ErrNotExist) {
		return writeNew(file.path, content)
	}
	return replaceFile(file.path, content)
}

// apply makes one edit in the workspace; nothing is written until every edit has gone through.
func (w *workspace) apply(edit domain.Edit) error {
	switch change := edit.(type) {
	case domain.SetAccountTag:
		setTag(w.sources, change)
	case domain.RemoveAccountTag:
		editDirective(w.sources, change.Account, func(tags []string) []string { return withoutTag(tags, change.Key) })
	case domain.DeleteRule:
		return deleteRule(w.sources, change.Pos)
	case domain.Declare:
		declare(w.sources, change)
	case domain.AddTransactions:
		return w.addTransactions(change)
	case domain.SetCategoryRule:
		return w.setCategoryRule(change)
	case domain.SetCarry:
		return w.setCarry(change)
	case domain.AddRule:
		return addRule(w.sources, change)
	case domain.ReplaceRule:
		return replaceRule(w.sources, change)
	}
	return nil
}

// ruleAt is the file and the first line index of the `~` block that starts at pos. The line there must be a
// rule line, or the journal changed under konto.
func ruleAt(sources []*source, pos domain.SourcePos) (file *source, first int, err error) {
	for _, source := range sources {
		if source.path != pos.File {
			continue
		}
		first = pos.Line - 1
		if first < 0 || first >= len(source.lines) || !strings.HasPrefix(source.lines[first], "~") {
			return nil, 0, fmt.Errorf("%s:%d is no longer a periodic rule: reload and try again", pos.File, pos.Line)
		}
		return source, first, nil
	}
	return nil, 0, fmt.Errorf("%s is not part of the journal", pos.File)
}

// deleteRule removes the `~` block that starts at pos.
func deleteRule(sources []*source, pos domain.SourcePos) error {
	file, first, err := ruleAt(sources, pos)
	if err != nil {
		return err
	}
	file.lines = slices.Delete(file.lines, first, blockEnd(file.lines, first))
	file.lines = withoutDoubleBlank(file.lines, first)
	file.dirty = true
	return nil
}

// replaceRule puts the lines in place of the `~` block that starts at pos.
func replaceRule(sources []*source, edit domain.ReplaceRule) error {
	file, first, err := ruleAt(sources, edit.Pos)
	if err != nil {
		return err
	}
	file.lines = slices.Replace(file.lines, first, blockEnd(file.lines, first), edit.Lines...)
	file.dirty = true
	return nil
}

// blockEnd is the index after the block that starts at first: its line and the indented lines under it.
func blockEnd(lines []string, first int) int {
	end := first + 1
	for end < len(lines) && strings.TrimLeft(lines[end], " \t") != lines[end] {
		end++
	}
	return end
}

// withoutDoubleBlank drops one of two blank lines that a removal at index at brought together; the
// last of them can be the file's final newline, so either goes without telling the difference.
func withoutDoubleBlank(lines []string, at int) []string {
	if at == 0 || at >= len(lines) || lines[at-1] != "" || lines[at] != "" {
		return lines
	}
	return slices.Delete(lines, at, at+1)
}

// setTag puts the tag on the account's directive, or declares the account in the main journal.
func setTag(sources []*source, edit domain.SetAccountTag) {
	tag := edit.Key + ":" + edit.Value
	update := func(tags []string) []string { return append(withoutTag(tags, edit.Key), tag) }
	if editDirective(sources, edit.Account, update) {
		return
	}
	appendLine(sources[0], "account "+edit.Account+"  ; "+tag)
}

// editDirective rewrites the tags of the account's directive; false when no file declares it.
func editDirective(sources []*source, account string, update func(tags []string) []string) bool {
	for _, source := range sources {
		for i, line := range source.lines {
			name, ok := directiveName(line)
			if !ok || name != account {
				continue
			}
			rest, _ := strings.CutPrefix(line, "account ")
			head, comment, _ := strings.Cut(rest, ";")
			source.lines[i] = "account " + strings.TrimRight(head, " \t") + joinTags(update(splitTags(comment)))
			source.dirty = true
			return true
		}
	}
	return false
}

// directiveName is the account an `account` directive line declares.
func directiveName(line string) (string, bool) {
	declaration, ok := parseDeclaration(line)
	return declaration.Name, ok
}

func splitTags(comment string) []string {
	var tags []string
	for tag := range strings.SplitSeq(comment, ",") {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "  ; " + strings.Join(tags, ", ")
}

// withoutTag drops the tags with the given key.
func withoutTag(tags []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(tags), func(tag string) bool { return strings.HasPrefix(tag, key+":") })
}

// ownerWrite is the permission bit that lets the owner write a file.
const ownerWrite = 0o200

// replaceFile writes content to path through a temp file beside it, keeping its mode.
func replaceFile(path, content string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if info.Mode().Perm()&ownerWrite == 0 { // the rename below would replace even a read-only file
		return fmt.Errorf("writing %s: %w", path, fs.ErrPermission)
	}
	temp := path + ".konto-tmp"
	if err := os.WriteFile(temp, []byte(content), info.Mode().Perm()); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
