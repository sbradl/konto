package main

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/carry"
	"github.com/sbradl/konto/internal/config"
	"github.com/sbradl/konto/internal/journal"
	"github.com/sbradl/konto/internal/ui"
)

func builtPorts(env map[string]string) ui.Ports {
	return buildPorts(config.Config{JournalPath: "/data/main.journal", Home: "/home/me"}, envOf(env), time.Now)
}

// The UI writes through the carry writer, which prepares the carry once, and the Verifier must render that prepared
// changeset through the same journal file the writer writes to: rendering through the carry writer again would
// count the carry twice, and through another file would check a journal that is not written.
func TestBuildPortsVerifiesTheJournalTheWriterWrites(t *testing.T) {
	built := builtPorts(nil)
	written := reflect.ValueOf(built.Writer)
	render := reflect.ValueOf(built.Verifier).Elem().FieldByName("render").Elem() // the Verifier's renderer

	if written.Type() != reflect.TypeFor[*carry.Writer]() {
		t.Fatalf("the UI writes through %s, want the carry writer", written.Type())
	}
	file := written.Elem().FieldByName("file").Elem() // the file the carry writer writes to
	if render.Type() != reflect.TypeFor[journal.File]() || file.Type() != render.Type() ||
		render.FieldByName("path").String() != file.FieldByName("path").String() {
		t.Errorf("the Verifier renders through %s, want the journal file the carry writer writes to", render.Type())
	}
}

// The editor and the file picker's start follow the environment.
func TestBuildPortsTakeTheEditorAndDownloadsFromTheEnvironment(t *testing.T) {
	built := builtPorts(map[string]string{"EDITOR": "nano"})

	if got := built.Editor("/data/2026.journal", 3).Args; !slices.Equal(got, []string{"nano", "+3", "/data/2026.journal"}) {
		t.Errorf("editor args = %v", got)
	}
	if built.DownloadDir != "/home/me/Downloads" {
		t.Errorf("download dir = %q", built.DownloadDir)
	}
}
