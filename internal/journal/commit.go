package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ownerWrite is the permission bit that lets the owner write a file.
const ownerWrite = 0o200

// stagedFile is a file's new content, written to a temp file beside it, waiting to take its place.
type stagedFile struct {
	path, temp string
}

// commitAll writes the files whole or not at all, as far as the file system allows: every new content is
// staged beside its file first, and only once all are staged do renames put them in place. A failed stage
// leaves every file as it was. Only a rename failing after an earlier one went through (rare: same
// directory, nothing left to write) can leave the changeset half applied; files come in an order that keeps
// such a half harmless (see workspace.dirty).
func commitAll(files []*source) error {
	staged := make([]stagedFile, 0, len(files))
	for _, file := range files {
		next, err := stage(file)
		if err != nil {
			discard(staged)
			return err
		}
		staged = append(staged, next)
	}
	for i, file := range staged {
		if err := os.Rename(file.temp, file.path); err != nil {
			discard(staged[i:])
			return fmt.Errorf("writing %s: %w", file.path, err)
		}
	}
	return nil
}

// stage writes the source's lines to a temp file beside it, with the mode the file has, or a new file's
// mode when it is not there yet. A read-only file is refused: the rename would replace it all the same.
func stage(file *source) (stagedFile, error) {
	mode := fs.FileMode(fileMode)
	info, err := os.Stat(file.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return stagedFile{}, fmt.Errorf("writing %s: %w", file.path, err)
	}
	if err == nil {
		if info.Mode().Perm()&ownerWrite == 0 {
			return stagedFile{}, fmt.Errorf("writing %s: %w", file.path, fs.ErrPermission)
		}
		mode = info.Mode().Perm()
	}
	staged := stagedFile{path: file.path, temp: file.path + ".konto-tmp"}
	if err := os.WriteFile(staged.temp, []byte(strings.Join(file.lines, "\n")), mode); err != nil {
		return stagedFile{}, fmt.Errorf("writing %s: %w", file.path, err)
	}
	return staged, nil
}

// discard removes the staged temp files; the files they were for stay as they were.
func discard(staged []stagedFile) {
	for _, file := range staged {
		_ = os.Remove(file.temp)
	}
}
