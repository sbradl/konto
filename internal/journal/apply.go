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
		switch e := edit.(type) {
		case domain.SetAccountTag:
			setTag(sources, e)
		case domain.RemoveAccountTag:
			editDirective(sources, e.Account, func(tags []string) []string { return withoutTag(tags, e.Key) })
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
