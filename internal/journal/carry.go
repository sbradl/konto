package journal

import (
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// CarryYears are the years whose carry the changeset leaves stale (ADR 0013): every year with a file after the
// earliest year written to, loaded or not, and a year file the changeset starts, as long as an earlier year
// file is there to carry from. Oldest first, so each carry is computed on top of the one before.
func (f File) CarryYears(changeset domain.Changeset) ([]int, error) {
	work, err := f.plan(changeset)
	if err != nil {
		return nil, err
	}

	if len(work.touched) == 0 {
		return nil, nil
	}
	earliest := slices.Min(work.touched)

	return slices.DeleteFunc(work.years(), func(year int) bool { return !work.carryStale(year, earliest) }), nil
}

// carryStale reports whether the carry into year goes stale when earliest is the earliest year written to:
// the year comes after it, or the changeset starts the year's file, and an earlier year file is there to
// carry from.
func (w *workspace) carryStale(year, earliest int) bool {
	startedOrLater := year > earliest || (year == earliest && w.created[year])

	return startedOrLater && w.priorYear(year) != 0
}

// years are the years that have a file, on disk or started by the edits, oldest first.
func (w *workspace) years() []int {
	years := slices.Clone(w.onDisk)
	for year := range w.created {
		years = append(years, year)
	}
	slices.Sort(years)

	return slices.Compact(years)
}

func (w *workspace) findYear(year int) *source {
	path := w.yearPath(year)
	for _, s := range w.sources {
		if s.path == path {
			return s
		}
	}

	return nil
}

// readYear reads the year's file from disk into the workspace.
func (w *workspace) readYear(year int) (*source, error) {
	seen := map[string]bool{}
	for _, s := range w.sources {
		seen[s.path] = true
	}

	if err := readSource(w.yearPath(year), seen, &w.sources); err != nil {
		return nil, err
	}

	return w.findYear(year), nil
}

// priorYear is the latest year before year that has a file; 0 when there is none.
func (w *workspace) priorYear(year int) int {
	earlier := slices.DeleteFunc(w.years(), func(y int) bool { return y >= year })
	if len(earlier) == 0 {
		return 0
	}

	return earlier[len(earlier)-1]
}

// setCarry replaces the managed carry into the edit's year.
func (w *workspace) setCarry(edit domain.SetCarry) error {
	opening, err := w.yearFile(edit.Year)
	if err != nil {
		return err
	}
	removeCarry(opening, firstDayOf(edit.Year))

	prior := w.priorYear(edit.Year)
	if prior == 0 {
		return nil
	}

	closing, err := w.yearFile(prior)
	if err != nil {
		return err
	}
	removeCarry(closing, lastDayOf(prior))

	if len(edit.Balances) == 0 {
		return nil
	}
	appendBlock(closing, edit.Closing(lastDayOf(prior)).Lines())
	prependBlock(opening, edit.Opening(firstDayOf(edit.Year)).Lines())

	return nil
}

func firstDayOf(year int) time.Time { return time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC) }

func lastDayOf(year int) time.Time { return time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC) }

// removeCarry drops the managed carry transaction dated date from the file.
func removeCarry(file *source, date time.Time) {
	for i := 0; i < len(file.lines); i++ {
		if isCarryHeader(file.lines[i], date) {
			removeBlockAt(file, i)
			i-- // the line after the removed block is now at i
		}
	}
}

// isCarryHeader reports whether the line opens the managed carry transaction dated date.
func isCarryHeader(line string, date time.Time) bool {
	when, ok := headerDate(line)

	return ok && when.Equal(date) && strings.Contains(line, domain.CarryTag)
}

// removeBlockAt removes the transaction that starts at index first, and the blank line it leaves doubled
// or, at the top of the file, leading.
func removeBlockAt(file *source, first int) {
	file.lines = slices.Delete(file.lines, first, blockEnd(file.lines, first))

	file.lines = withoutDoubleBlank(file.lines, first)
	if first == 0 && len(file.lines) > 0 && file.lines[0] == "" {
		file.lines = file.lines[1:]
	}
	file.dirty = true
}

// prependBlock puts lines at the very start of the file, before any transaction of the same day.
func prependBlock(file *source, block []string) {
	if len(file.lines) == 1 && file.lines[0] == "" {
		file.lines = append(slices.Clone(block), "")
	} else {
		file.lines = slices.Insert(file.lines, 0, append(slices.Clone(block), "")...)
	}
	file.dirty = true
}
