package imports

import (
	"path/filepath"
	"strings"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/layout"
	"github.com/sbradl/konto/internal/ui/theme"
)

// pickerRows are the picker's entries: the parent folder first, unless the folder is the root.
func (m Model) pickerRows() []domain.FileEntry {
	if filepath.Dir(m.dir) == m.dir {
		return m.entries
	}
	return append([]domain.FileEntry{{Name: parentEntry, IsDir: true}}, m.entries...)
}

func (m Model) pickerPage() int { return max(1, m.height-panelChrome-1) }

func (m Model) pickerKey(key string) (Model, Action) {
	rows := m.pickerRows()
	switch key {
	case keyEsc:
		m.picking = false
	case keyEnter:
		if len(rows) > 0 {
			return m, m.activate(rows[m.browse.Row])
		}
	default:
		m.browse = m.browse.Move(key, len(rows), m.pickerPage())
	}
	return m, Action{}
}

// activate is what Enter does on an entry: go into a folder, or open a file.
func (m Model) activate(entry domain.FileEntry) Action {
	switch {
	case entry.Name == parentEntry:
		return Action{Browse: filepath.Dir(m.dir)}
	case entry.IsDir:
		return Action{Browse: filepath.Join(m.dir, entry.Name)}
	}
	return Action{Open: filepath.Join(m.dir, entry.Name)}
}

func (m Model) scrollPicker() Model {
	next := m
	next.browse = m.browse.Scrolled(m.pickerPage())
	return next
}

// picker is the file picker: the directory, then its folders and CSV files with the cursor.
func (m Model) picker(height int) string {
	lines := []string{" " + theme.Bold.Render(m.dir)}
	if m.listErr != nil {
		lines = append(lines, " "+theme.Negative.Render(m.listErr.Error()))
		return layout.Panel(pickerTitle, m.width, height, strings.Join(lines, "\n"))
	}
	rows := m.pickerRows()
	for i := m.browse.Offset; i < min(len(rows), m.browse.Offset+m.pickerPage()); i++ {
		name := rows[i].Name
		if rows[i].IsDir && name != parentEntry {
			name += "/"
		}
		lines = append(lines, listLine(name, i, m.browse.Row, m.innerWidth()))
	}
	return layout.Panel(pickerTitle, m.width, height, strings.Join(lines, "\n"))
}
