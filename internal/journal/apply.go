package journal

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// workspace is the journal's files as edits change them, and the categories.rules file beside the journal.
type workspace struct {
	sources   []*source
	rulesPath string
	rules     *source // read when an edit first needs it
	thisYear  int     // nothing is written after it
	yearPath  func(year int) string
	onDisk    []int        // the years that have a file, loaded or not
	touched   []int        // the years transactions were added to
	created   map[int]bool // the years whose file the edits started
}

// plan makes the edits in memory; nothing is written.
func (f File) plan(changeset domain.Changeset) (*workspace, error) {
	sources, err := f.sources()
	if err != nil {
		return nil, err
	}
	work := &workspace{
		sources: sources, rulesPath: f.RulesPath(),
		thisYear: f.thisYear(), yearPath: f.yearPath, onDisk: f.yearsOnDisk(), created: map[int]bool{},
	}
	for _, edit := range changeset.Edits {
		if err := work.apply(edit); err != nil {
			return nil, err
		}
	}
	return work, nil
}

// dirty is the sources the edits changed, in the order they are put in place: categories.rules, then the
// main journal and its includes (directives), then the year files (transactions). Rules and directives
// are harmless without the transactions they were written for, so a write cut short leaves no broken file.
func (w *workspace) dirty() []*source {
	changed := slices.DeleteFunc(slices.Clone(w.sources), func(s *source) bool { return !s.dirty })
	if w.rules != nil && w.rules.dirty {
		changed = slices.Insert(changed, 0, w.rules)
	}
	return changed
}

// Apply writes a changeset into the journal's files and categories.rules, all of them or none (see
// commitAll); lines the edits do not name stay byte for byte as they were.
func (f File) Apply(changeset domain.Changeset) error {
	work, err := f.plan(changeset)
	if err != nil {
		return err
	}
	return commitAll(work.dirty())
}

// Render is the journal as it would read after the changeset, its included files spliced in where they are
// included, then the year files the changeset reads or starts (older, unloaded ones it touches among them)
// that the main journal does not include itself, so hledger can check it from standard input without anything
// being written.
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
	writeFile := func(content string) {
		text.WriteString(content)
		if !strings.HasSuffix(content, "\n") { // a file without its final newline would run into the next
			text.WriteString("\n")
		}
	}
	spliced := map[string]bool{}
	writeFile(flatten(byPath, work.sources[0], spliced))
	for _, year := range work.years() {
		if file := work.findYear(year); file != nil && !spliced[file.path] {
			writeFile(strings.Join(file.lines, "\n"))
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
		if _, ok := directiveArgs(line, "include"); !ok {
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

// apply makes one edit in the workspace; nothing is written until every edit has gone through. Every kind of
// domain.Edit has its case (TestApplyHandlesEveryKindOfEdit): only a type that embeds the interface falls
// through.
func (w *workspace) apply(edit domain.Edit) error {
	switch change := edit.(type) {
	case domain.SetAccountTag:
		setTag(w.sources, change)
		return nil
	case domain.RemoveAccountTag:
		editDirective(w.sources, change.Account, func(tags []string) []string { return withoutTag(tags, change.Key) })
		return nil
	case domain.Declare:
		declare(w.sources, change)
		return nil
	case domain.AddRule:
		addRule(w.sources, change)
		return nil
	case domain.AddTransactions:
		return w.addTransactions(change)
	case domain.SetCategoryRule:
		return w.setCategoryRule(change)
	case domain.SetCarry:
		return w.setCarry(change)
	case domain.DeleteRule:
		return deleteRule(w.sources, change.Pos)
	case domain.ReplaceRule:
		return replaceRule(w.sources, change)
	}
	return fmt.Errorf("unsupported edit %T", edit)
}

// ruleAt is the file and the first line index of the `~` block that starts at pos. The line there must be a
// rule line, or the journal changed under konto.
func ruleAt(sources []*source, pos domain.SourcePos) (file *source, first int, err error) {
	for _, source := range sources {
		if source.path != pos.File {
			continue
		}
		first = pos.Line - 1
		if first < 0 || first >= len(source.lines) || !isRuleHeader(source.lines[first]) {
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
	for end < len(lines) && indented(lines[end]) {
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
	appendLine(sources[0], domain.DeclareAccount(edit.Account, tag).Line())
}

// editDirective rewrites the tags of the account's directive; false when no file declares it.
func editDirective(sources []*source, account string, update func(tags []string) []string) bool {
	for _, source := range sources {
		for i, line := range source.lines {
			directive, ok := parseDirective(line, domain.DirectiveAccount)
			if !ok || directive.name != account {
				continue
			}
			tags := update(splitTags(directive.comment))
			source.lines[i] = domain.DirectiveAccount + " " + directive.head + joinTags(tags)
			source.dirty = true
			return true
		}
	}
	return false
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

func joinTags(tags []string) string { return domain.TrailingComment(strings.Join(tags, ", ")) }

// withoutTag drops the tags with the given key.
func withoutTag(tags []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(tags), func(tag string) bool { return strings.HasPrefix(tag, key+":") })
}
