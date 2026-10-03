package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// source is one journal file of the include set, as lines. Joining the lines with "\n"
// restores the file byte for byte.
type source struct {
	path  string
	lines []string
	dirty bool
}

// sources reads the main journal and every file it includes, each once, a file before
// the files it includes. Include patterns are relative to the including file; one that
// matches nothing is skipped, as hledger does for globs.
func (f File) sources() ([]*source, error) {
	var sources []*source
	if err := readSource(f.path, map[string]bool{}, &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// readSource reads path into sources, then the files it includes; seen guards against include cycles.
func readSource(path string, seen map[string]bool, sources *[]*source) error {
	if seen[path] {
		return nil
	}
	seen[path] = true
	text, err := os.ReadFile(path) //nolint:gosec // the journal path is the user's own configuration
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	current := &source{path: path, lines: strings.Split(string(text), "\n")}
	*sources = append(*sources, current)
	for _, line := range current.lines {
		matches, err := includedFiles(path, line)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if err := readSource(match, seen, sources); err != nil {
				return err
			}
		}
	}
	return nil
}

// includedFiles are the files an `include` line of the file at from names; none for any other line.
func includedFiles(from, line string) ([]string, error) {
	pattern, ok := strings.CutPrefix(line, "include ")
	if !ok {
		return nil, nil
	}
	pattern = strings.TrimSpace(pattern)
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(filepath.Dir(from), pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("including %s: %w", pattern, err)
	}
	return matches, nil
}
