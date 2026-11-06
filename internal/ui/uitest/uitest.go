// Package uitest holds the helpers the ui packages' tests share: plain text of a styled screen and golden files.
package uitest

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// escapes are the SGR sequences lipgloss styles text with.
var escapes = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

// Plain is s without its styling.
func Plain(s string) string { return escapes.ReplaceAllString(s, "") }

// The golden files' folder and file permissions.
const (
	dirMode  = 0o750
	fileMode = 0o600
)

// Golden compares got, plus a final newline, with testdata/<name>.txt; KONTO_UPDATE_GOLDEN=1 rewrites the file.
func Golden(tb testing.TB, name, got string) {
	tb.Helper()
	path := filepath.Join("testdata", name+".txt")

	got += "\n"
	if os.Getenv("KONTO_UPDATE_GOLDEN") != "" {
		if err := errors.Join(os.MkdirAll("testdata", dirMode), os.WriteFile(path, []byte(got), fileMode)); err != nil {
			tb.Fatal(err)
		}
	}

	want, err := os.ReadFile(path) //nolint:gosec // the path is the test's own golden file
	if err != nil {
		tb.Fatal(err)
	}

	if got != string(want) {
		tb.Errorf("%s differs from %s:\n%s\nwant\n%s", name, path, got, want)
	}
}
