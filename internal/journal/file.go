package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// skeleton is a new journal: the declarations konto's own writes rely on, so
// the journal passes every hledger check from the start.
const skeleton = `decimal-mark ,
commodity 1.000,00 EUR
account equity:opening-balances
account assets:transfers  ; konto:source:clearing
tag konto
tag konto-rule
`

const (
	dirMode  = 0o755
	fileMode = 0o644
)

// File is the main journal file on disk.
type File struct {
	path string
}

// NewFile returns the journal file at path.
func NewFile(path string) File { return File{path: path} }

// Exists reports whether the journal file is there.
func (f File) Exists() (bool, error) {
	_, err := os.Stat(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", f.path, err)
	}
	return true, nil
}

// Create writes the skeleton journal, making parent directories as needed, and
// an empty categories.rules beside it. It never overwrites an existing journal
// or rules file.
func (f File) Create() error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	if err := writeNew(f.path, skeleton); err != nil {
		return err
	}
	return touch(filepath.Join(dir, "categories.rules"))
}

// writeNew creates path with content, failing if it already exists.
func writeNew(path, content string) error {
	//nolint:gosec // path derives from the configured journal
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	_, writeErr := file.WriteString(content)
	return errors.Join(writeErr, file.Close())
}

// touch makes sure a file exists at path, leaving its content alone.
func touch(path string) error {
	//nolint:gosec // path derives from the configured journal
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	return file.Close() //nolint:wrapcheck // closing a file we just opened; the path is not in play
}
