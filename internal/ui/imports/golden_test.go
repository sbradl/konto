package imports

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
)

// golden compares the plain view with testdata/<name>.txt; KONTO_UPDATE_GOLDEN=1 rewrites the file.
func golden(t *testing.T, name string, model Model) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")
	got := screen(model) + "\n"
	if os.Getenv("KONTO_UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("%s differs from %s:\n%s\nwant\n%s", name, path, got, want)
	}
}

func bigImport(rows, skipped int) csvimport.Staged {
	big := staged()
	for i := range rows {
		big.Rows = append(big.Rows, csvimport.Row{Date: day("2026-09-20"), Description: fmt.Sprintf("Row %02d", i), Amount: eur(-100)})
	}
	for i := range skipped {
		big.Skipped = append(big.Skipped, csvimport.Row{Date: day("2026-09-01"), Description: fmt.Sprintf("Old %02d", i), Amount: eur(-100)})
	}
	return big
}

func TestTheStagedViewIsLaidOutAsTheGoldenFilesShow(t *testing.T) {
	golden(t, "staged-180x40", opened())
	golden(t, "staged-100x20", New("/dl").Resize(100, 20).WithStaged(staged()))
	for _, height := range []int{12, 16, 19} {
		golden(t, fmt.Sprintf("staged-100x%d", height), New("/dl").Resize(100, height).WithStaged(bigImport(10, 10)))
	}
	golden(t, "staged-banner", opened().WithMessage("the file is not a Postbank export"))
	golden(t, "empty-banner", New("/dl").Resize(100, 20).WithMessage("nothing opened"))

	probe := New("/dl").Resize(100, 30).WithStaged(staged())
	_, skippedRoom := probe.layoutHeights()
	capacity := skippedRoom - layout.PanelChrome
	for _, skipped := range []int{capacity - 1, capacity, capacity + 1} {
		golden(t, fmt.Sprintf("skipped-%d-of-%d", skipped, capacity), New("/dl").Resize(100, 30).WithStaged(bigImport(0, skipped-2)))
	}
}

func TestLongTablesScrollAsTheGoldenFilesShow(t *testing.T) {
	m := New("/dl").Resize(100, 30).WithStaged(bigImport(40, 40))
	golden(t, "long-top", m)
	for range 5 {
		m, _ = m.Key("down")
	}
	golden(t, "long-fifth", m)
	for range 25 {
		m, _ = m.Key("down")
	}
	golden(t, "long-thirtieth", m)
	end, _ := m.Key("end")
	golden(t, "long-end", end)
	paged, _ := end.Key("pgup")
	golden(t, "long-paged-up", paged)
}

func TestThePickerIsLaidOutAsTheGoldenFilesShow(t *testing.T) {
	entries := make([]domain.FileEntry, 0, 60)
	for i := range 60 {
		entries = append(entries, domain.FileEntry{Name: fmt.Sprintf("f%02d.csv", i)})
	}
	m, _ := New("/dl").Resize(100, 20).Key("n")
	m = m.WithListing("/dl", entries, nil)
	golden(t, "picker-top", m)
	for range 25 {
		m, _ = m.Key("down")
	}
	golden(t, "picker-25", m)
	end, _ := m.Key("end")
	golden(t, "picker-end", end)
}

func TestTheDialogsAreLaidOutAsTheGoldenFilesShow(t *testing.T) {
	golden(t, "dialog-choices", onRow(categorising(), 2))
	golden(t, "dialog-choices-second", keys(onRow(categorising(), 2), "down"))
	golden(t, "dialog-form-pattern", newRule(t))
	golden(t, "dialog-form-purpose", keys(newRule(t), "tab"))
	golden(t, "dialog-form-category", keys(newRule(t), "tab", "tab"))
	golden(t, "dialog-form-narrow", keys(onRow(New("/dl").Resize(40, 30).WithStaged(categorisable()), 2), "enter"))
	golden(t, "dialog-rules", keys(onRow(categorising(), 2), "down", "enter"))

	many := categorisable()
	for i := range 40 {
		many.Known = append(many.Known, domain.Account{Name: "expenses:category" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	short := New("/dl").Resize(120, 20).WithStaged(many)
	picking := keys(onRow(short, 2), "down", "down", "enter")
	golden(t, "dialog-picker-first", picking)
	golden(t, "dialog-picker-fifth", keys(picking, "down", "down", "down", "down", "down"))
	golden(t, "dialog-picker-end", keys(picking, "end"))

	fresh := categorisable()
	fresh.NewAccount, fresh.Account = true, domain.Account{}
	account := New("/dl").Resize(120, 30).WithStaged(fresh)
	golden(t, "dialog-account", account)
	golden(t, "dialog-account-refused", keys(typed(keys(account, "ctrl+u"), "assets:checking"), "enter"))
}
