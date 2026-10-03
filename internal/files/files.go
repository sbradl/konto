// Package files is the adapter to the plain files konto reads that are not its journal: bank exports
// and the directories they are picked from.
package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sbradl/konto/internal/domain"
)

// System reads from the real file system.
type System struct{}

// ReadFile is the content of the file at path.
func (System) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the user picked this file
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return data, nil
}

// ReadDir lists the entries of the directory; a link to a folder counts as a folder.
func (System) ReadDir(dir string) ([]domain.FileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	listed := make([]domain.FileEntry, len(entries))
	for i, entry := range entries {
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			if target, err := os.Stat(filepath.Join(dir, entry.Name())); err == nil {
				isDir = target.IsDir()
			}
		}
		listed[i] = domain.FileEntry{Name: entry.Name(), IsDir: isDir}
	}
	return listed, nil
}
