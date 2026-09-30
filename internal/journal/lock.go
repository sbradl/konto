// Package journal is the adapter to the journal text files on disk. It is a
// targeted line reader/writer, never a parser (spec §4.2).
package journal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ErrAlreadyRunning means another konto holds the journal.
var ErrAlreadyRunning = errors.New("konto is already running on this journal")

const (
	lockFlags = os.O_CREATE | os.O_RDWR
	lockMode  = 0o600
)

// Lock is the single-instance lock on a journal (spec §4.8): an exclusive
// flock on a lock file next to it.
type Lock struct {
	file *os.File
}

// Acquire locks the journal at journalPath, or fails with ErrAlreadyRunning.
func Acquire(journalPath string) (*Lock, error) {
	// The lock file sits next to the journal konto was told to open.
	lockPath := journalPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), dirMode); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}
	file, err := os.OpenFile(lockPath, lockFlags, lockMode) //nolint:gosec // derived from the configured journal
	if err != nil {
		return nil, fmt.Errorf("creating lock file: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("%s: %w (%w)", journalPath, ErrAlreadyRunning, err)
	}
	return &Lock{file: file}, nil
}

// Release gives the journal up; closing the file drops the flock.
func (l *Lock) Release() error {
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("releasing lock: %w", err)
	}
	return nil
}
