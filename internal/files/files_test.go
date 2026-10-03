package files

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

func TestReadFileReturnsTheBytesAndNamesTheFileOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.csv")
	if err := os.WriteFile(path, []byte("x;y"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := System{}.ReadFile(path)
	if err != nil || string(got) != "x;y" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := (System{}).ReadFile(path + ".missing"); err == nil {
		t.Errorf("expected an error for a missing file")
	}
}

func TestReadDirListsFilesFoldersAndLinksToFolders(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sept.csv", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "subdir"), filepath.Join(dir, "link-to-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "sept.csv"), filepath.Join(dir, "link-to-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "broken")); err != nil {
		t.Fatal(err)
	}

	got, err := System{}.ReadDir(dir)
	slices.SortFunc(got, func(a, b domain.FileEntry) int { return compare(a.Name, b.Name) })

	want := []domain.FileEntry{
		{Name: "broken"},
		{Name: "link-to-dir", IsDir: true},
		{Name: "link-to-file"},
		{Name: "notes.txt"},
		{Name: "sept.csv"},
		{Name: "subdir", IsDir: true},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %+v, %v; want %+v", got, err, want)
	}
	if _, err := (System{}).ReadDir(filepath.Join(dir, "missing")); err == nil {
		t.Errorf("expected an error for a missing directory")
	}
}

func compare(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
