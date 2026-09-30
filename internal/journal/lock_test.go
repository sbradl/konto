package journal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func journalIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "main.journal")
}

// While one konto holds a journal, a second one is refused.
func TestSecondAcquireIsRefused(t *testing.T) {
	path := journalIn(t)
	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	_, err = Acquire(path)

	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Acquire error = %v, want ErrAlreadyRunning", err)
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error should say konto is already running, got: %v", err)
	}
}

// Releasing the lock lets the next konto in.
func TestAcquireSucceedsAfterRelease(t *testing.T) {
	path := journalIn(t)
	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	second, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after Release failed: %v", err)
	}
	_ = second.Release()
}

// Different journals do not block each other.
func TestDifferentJournalsLockIndependently(t *testing.T) {
	first, err := Acquire(journalIn(t))
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := Acquire(journalIn(t))
	if err != nil {
		t.Fatalf("Acquire on another journal failed: %v", err)
	}
	_ = second.Release()
}

// A first run may lock a journal whose directory does not exist yet.
func TestAcquireCreatesMissingDirectory(t *testing.T) {
	lock, err := Acquire(filepath.Join(t.TempDir(), "a", "b", "main.journal"))
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	_ = lock.Release()
}

// A lock file that cannot be created is an error, not a silent pass.
func TestAcquireFailsWhenLockFileCannotBeCreated(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Acquire(filepath.Join(blocker, "main.journal"))

	if err == nil || errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("expected a plain creation error, got: %v", err)
	}
}

// Releasing a lock twice reports the second, failing close.
func TestReleaseTwiceFails(t *testing.T) {
	lock, err := Acquire(journalIn(t))
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("first Release failed: %v", err)
	}

	if err := lock.Release(); err == nil {
		t.Error("second Release should fail, got nil")
	}
}

// A lock file that cannot be opened is an error, not a silent pass.
func TestAcquireFailsWhenLockFileCannotBeOpened(t *testing.T) {
	path := journalIn(t)
	if err := os.Mkdir(path+".lock", 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Acquire(path)

	if err == nil || errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("expected a plain open error, got: %v", err)
	}
}
