package journal

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// Apply writes a changeset into the journal's files. Each touched file is replaced in one go
// (temp file, then rename), so a failed write never leaves a half-edited journal; lines the
// edits do not name stay byte for byte as they were.
func (f File) Apply(changeset domain.Changeset) error {
	sources, err := f.sources()
	if err != nil {
		return err
	}
	for _, edit := range changeset.Edits {
		if err := applyEdit(sources, edit); err != nil {
			return err
		}
	}
	for _, source := range sources {
		if source.dirty {
			if err := replaceFile(source.path, strings.Join(source.lines, "\n")); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyEdit makes one edit in the sources; nothing is written until every edit has gone through.
func applyEdit(sources []*source, edit domain.Edit) error {
	switch e := edit.(type) {
	case domain.SetAccountTag:
		setTag(sources, e)
	case domain.RemoveAccountTag:
		editDirective(sources, e.Account, func(tags []string) []string { return withoutTag(tags, e.Key) })
	case domain.DeleteRule:
		return deleteRule(sources, e.Pos)
	}
	return nil
}

// deleteRule removes the `~` block that starts at pos. The line there must be a rule line, or the
// journal changed under konto and nothing is deleted.
func deleteRule(sources []*source, pos domain.SourcePos) error {
	for _, source := range sources {
		if source.path != pos.File {
			continue
		}
		first := pos.Line - 1
		if first < 0 || first >= len(source.lines) || !strings.HasPrefix(source.lines[first], "~") {
			return fmt.Errorf("%s:%d is no longer a periodic rule: reload and try again", pos.File, pos.Line)
		}
		source.lines = slices.Delete(source.lines, first, blockEnd(source.lines, first))
		source.lines = withoutDoubleBlank(source.lines, first)
		source.dirty = true
		return nil
	}
	return fmt.Errorf("%s is not part of the journal", pos.File)
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
	main := sources[0]
	line := "account " + edit.Account + "  ; " + tag
	if last := len(main.lines) - 1; main.lines[last] == "" { // keep the file's final newline last
		main.lines = slices.Insert(main.lines, last, line)
	} else {
		main.lines = append(main.lines, line)
	}
	main.dirty = true
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

// replaceFile writes content to path through a temp file beside it, keeping its mode.
func replaceFile(path, content string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
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
