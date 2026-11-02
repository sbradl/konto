package uitest

import (
	"os"
	"path/filepath"
	"testing"
)

// recorder is a testing.TB that notes failures instead of stopping the test.
type recorder struct {
	testing.TB
	fatal, failed bool
}

func (*recorder) Helper()                 {}
func (r *recorder) Fatal(...any)          { r.fatal = true }
func (r *recorder) Errorf(string, ...any) { r.failed = true }

func TestPlainDropsTheStyling(t *testing.T) {
	if got := Plain("\x1b[1;38:5:2mbold\x1b[m text"); got != "bold text" {
		t.Errorf("Plain = %q", got)
	}
}

func TestGoldenPassesOnTheSameTextAndFailsOnOther(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("testdata", "a.txt"), []byte("same\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	same, other := &recorder{TB: t}, &recorder{TB: t}
	Golden(same, "a", "same")
	Golden(other, "a", "other")
	if same.fatal || same.failed || other.fatal || !other.failed {
		t.Errorf("same %+v, other %+v", *same, *other)
	}
}

func TestGoldenFailsWithoutItsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	missing := &recorder{TB: t}
	Golden(missing, "a", "text")
	if !missing.fatal {
		t.Error("a missing golden file did not fail")
	}
}

func TestGoldenRewritesTheFileWhenAskedTo(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KONTO_UPDATE_GOLDEN", "1")
	rewritten := &recorder{TB: t}
	Golden(rewritten, "a", "text")
	written, err := os.ReadFile(filepath.Join("testdata", "a.txt"))
	if rewritten.fatal || rewritten.failed || string(written) != "text\n" {
		t.Errorf("rewrite %+v wrote %q, %v", *rewritten, written, err)
	}
}

func TestGoldenFailsWhenTheFileCannotBeWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("KONTO_UPDATE_GOLDEN", "1")
	if err := os.WriteFile("testdata", nil, 0o644); err != nil { // a file where the folder would go
		t.Fatal(err)
	}
	blocked := &recorder{TB: t}
	Golden(blocked, "a", "text")
	if !blocked.fatal {
		t.Error("an unwritable golden file did not fail")
	}
}
