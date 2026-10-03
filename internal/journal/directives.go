package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sbradl/konto/internal/domain"
)

// AccountDeclarations reads the `account` directives of the journal and the files it
// includes. It is a line reader, not a parser: it knows nothing of transactions.
func (f File) AccountDeclarations() ([]domain.Declaration, error) {
	return readDeclarations(f.path, map[string]bool{})
}

// readDeclarations reads one file, then follows its `include` lines; seen guards against include cycles.
func readDeclarations(path string, seen map[string]bool) ([]domain.Declaration, error) {
	if seen[path] {
		return nil, nil
	}
	seen[path] = true
	text, err := os.ReadFile(path) //nolint:gosec // the journal path is the user's own configuration
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var declarations []domain.Declaration
	for line := range strings.Lines(string(text)) {
		if declaration, ok := parseDeclaration(line); ok {
			declarations = append(declarations, declaration)
		}
		if pattern, ok := strings.CutPrefix(line, "include "); ok {
			included, err := readIncluded(path, strings.TrimSpace(pattern), seen)
			if err != nil {
				return nil, err
			}
			declarations = append(declarations, included...)
		}
	}
	return declarations, nil
}

// readIncluded reads the files an `include` line names, relative to the including file's directory.
func readIncluded(from, pattern string, seen map[string]bool) ([]domain.Declaration, error) {
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(filepath.Dir(from), pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("including %s: %w", pattern, err)
	}
	var declarations []domain.Declaration
	for _, match := range matches {
		included, err := readDeclarations(match, seen)
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, included...)
	}
	return declarations, nil
}

// parseDeclaration reads `account NAME  ; tag, tag` into a Declaration.
func parseDeclaration(line string) (domain.Declaration, bool) {
	rest, found := strings.CutPrefix(line, "account ")
	if !found {
		return domain.Declaration{}, false
	}
	head, comment, _ := strings.Cut(rest, ";")
	name, _, _ := strings.Cut(strings.TrimSpace(head), "  ")
	declaration := domain.Declaration{Name: strings.TrimSpace(name)}
	for tag := range strings.SplitSeq(comment, ",") {
		declaration.ApplyTag(strings.TrimSpace(tag))
	}
	return declaration, true
}
