package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
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

// File is the main journal file on disk, with the year files beside it (ADR 0013).
type File struct {
	path  string
	today func() time.Time // decides which years are the window
}

// NewFile returns the journal file at path; the window follows the clock.
func NewFile(path string) File { return File{path: path, today: time.Now} }

// WithToday returns the file with today as the day the window is counted from.
func (f File) WithToday(today func() time.Time) File {
	f.today = today
	return f
}

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

// Window is what hledger is given: the main journal plus last year's and this year's file when they
// exist, oldest first (ADR 0013). Files of older years stay on disk, unloaded.
func (f File) Window() []string {
	files := []string{f.path}
	for year := f.firstYear(); year <= f.lastYear(); year++ {
		if path := f.yearPath(year); isFile(path) {
			files = append(files, path)
		}
	}
	return files
}

// firstYear and lastYear bound the window: last year and this year.
func (f File) firstYear() int { return f.today().Year() - 1 }
func (f File) lastYear() int  { return f.today().Year() }

// yearPath is where the transactions of a year live.
func (f File) yearPath(year int) string {
	return filepath.Join(filepath.Dir(f.path), fmt.Sprintf("%04d.journal", year))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
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
