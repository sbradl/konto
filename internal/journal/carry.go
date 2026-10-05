package journal

import (
	"slices"
	"strings"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// CarryYears are the years whose carry the changeset leaves stale (ADR 0013): every loaded year after the
// earliest year written to, and a year file the changeset starts, as long as an earlier year file is loaded
// to carry from. Oldest first, so each carry is computed on top of the one before.
func (f File) CarryYears(changeset domain.Changeset) ([]int, error) {
	work, err := f.plan(changeset)
	if err != nil {
		return nil, err
	}
	if len(work.touched) == 0 {
		return nil, nil
	}
	earliest := slices.Min(work.touched)
	var years []int
	for year := work.firstYear; year <= work.lastYear; year++ {
		if work.hasYear(year) && work.priorYear(year) != 0 && (year > earliest || work.created[year]) {
			years = append(years, year)
		}
	}
	return years, nil
}

// hasYear reports a year file that is read or started.
func (w *workspace) hasYear(year int) bool { return w.findYear(year) != nil }

func (w *workspace) findYear(year int) *source {
	path := w.yearPath(year)
	if i := slices.IndexFunc(w.sources, func(s *source) bool { return s.path == path }); i >= 0 {
		return w.sources[i]
	}
	return nil
}

// priorYear is the latest loaded year before year that has a file; 0 when there is none.
func (w *workspace) priorYear(year int) int {
	for earlier := year - 1; earlier >= w.firstYear; earlier-- {
		if w.hasYear(earlier) {
			return earlier
		}
	}
	return 0
}

// setCarry replaces the managed carry into the edit's year.
func (w *workspace) setCarry(edit domain.SetCarry) error {
	opening, err := w.yearFile(edit.Year)
	if err != nil {
		return err
	}
	removeCarry(opening, time.Date(edit.Year, time.January, 1, 0, 0, 0, 0, time.UTC))
	prior := w.priorYear(edit.Year)
	if prior == 0 {
		return nil
	}
	closing, _ := w.yearFile(prior) // the prior year has its file
	removeCarry(closing, lastDayOf(prior))
	if len(edit.Balances) == 0 {
		return nil
	}
	appendBlock(closing, edit.Closing(lastDayOf(prior)).Lines())
	prependBlock(opening, edit.Opening(time.Date(edit.Year, time.January, 1, 0, 0, 0, 0, time.UTC)).Lines())
	return nil
}

func lastDayOf(year int) time.Time { return time.Date(year, time.December, 31, 0, 0, 0, 0, time.UTC) }

// removeCarry drops the managed carry transaction dated date from the file.
func removeCarry(file *source, date time.Time) {
	for i := 0; i < len(file.lines); i++ {
		when, ok := headerDate(file.lines[i])
		if !ok || !when.Equal(date) || !strings.Contains(file.lines[i], domain.CarryTag) {
			continue
		}
		file.lines = slices.Delete(file.lines, i, blockEnd(file.lines, i))
		file.lines = withoutDoubleBlank(file.lines, i)
		if i == 0 && len(file.lines) > 1 && file.lines[0] == "" {
			file.lines = file.lines[1:]
		}
		file.dirty = true
		i--
	}
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
