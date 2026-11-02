package carry

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

// fakeFile is a journal that is stale for the given years and renders a changeset as a line per edit.
type fakeFile struct {
	years     []int
	yearsErr  error
	renderErr error
	applyErr  error
	applied   []domain.Changeset
}

func (f *fakeFile) CarryYears(domain.Changeset) ([]int, error) { return f.years, f.yearsErr }

func (f *fakeFile) Render(changeset domain.Changeset) (string, error) {
	var lines []string
	for _, edit := range changeset.Edits {
		switch change := edit.(type) {
		case domain.SetCarry:
			lines = append(lines, "carry "+strconv.Itoa(change.Year)+" of "+strconv.Itoa(len(change.Balances)))
		case domain.Declare:
			lines = append(lines, "declare "+change.Name)
		default:
			lines = append(lines, "rows")
		}
	}
	return strings.Join(lines, ", "), f.renderErr
}

func (f *fakeFile) Apply(changeset domain.Changeset) error {
	f.applied = append(f.applied, changeset)
	return f.applyErr
}

type fakeBalances struct {
	byText map[string][]domain.Account
	err    error
	before []time.Time
}

func (f *fakeBalances) TextBalances(_ context.Context, text string, before time.Time) ([]domain.Account, error) {
	f.before = append(f.before, before)
	return f.byText[text], f.err
}

func checking(mantissa int64) []domain.Account {
	style := domain.DefaultAmount("EUR").Style
	amount := domain.Amount{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}
	return []domain.Account{{Name: "assets:checking", Balance: domain.MultiAmount{amount}}}
}

var rows = domain.Changeset{Edits: []domain.Edit{domain.AddTransactions{}}}

// carries wants the changeset to be the rows, the carry of each year, then what the carry transactions need declared.
func carries(years ...domain.SetCarry) domain.Changeset {
	edits := []domain.Edit{domain.AddTransactions{}}
	for _, year := range years {
		edits = append(edits, year)
	}
	return domain.Changeset{Edits: append(edits,
		domain.Declare{Directive: "account", Name: domain.OpeningBalanceAccount},
		domain.Declare{Directive: "payee", Name: domain.OpeningDescription},
		domain.Declare{Directive: "payee", Name: domain.ClosingDescription},
		domain.Declare{Directive: "tag", Name: "konto"},
	)}
}

// unordered is the changeset with its trailing declarations sorted: which directive is declared first means
// nothing.
func unordered(changeset domain.Changeset) domain.Changeset {
	edits := slices.Clone(changeset.Edits)
	first := len(edits)
	for first > 0 {
		if _, declares := edits[first-1].(domain.Declare); !declares {
			break
		}
		first--
	}
	slices.SortFunc(edits[first:], func(a, b domain.Edit) int {
		return strings.Compare(a.(domain.Declare).Line(), b.(domain.Declare).Line())
	})
	return domain.Changeset{Edits: edits}
}

func TestPrepareAddsTheCarryOfEveryStaleYearOldestFirst(t *testing.T) {
	file := &fakeFile{years: []int{2026, 2027}}
	gw := &fakeBalances{byText: map[string][]domain.Account{
		"rows, carry 2026 of 0":                  checking(900),
		"rows, carry 2026 of 1, carry 2027 of 0": checking(1200),
	}}

	got, err := NewWriter(file, gw).Prepare(t.Context(), rows)

	want := carries(domain.SetCarry{Year: 2026, Balances: checking(900)}, domain.SetCarry{Year: 2027, Balances: checking(1200)})
	if err != nil || !reflect.DeepEqual(unordered(got), unordered(want)) || file.applied != nil {
		t.Errorf("err = %v, prepared = %+v, want %+v; applied %+v", err, got, want, file.applied)
	}
	wantDays := []time.Time{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	if !reflect.DeepEqual(gw.before, wantDays) {
		t.Errorf("balances asked before %v, want %v", gw.before, wantDays)
	}
}

func TestChangesetsWithoutAStaleYearPassUntouched(t *testing.T) {
	gw := &fakeBalances{}

	got, err := NewWriter(&fakeFile{}, gw).Prepare(t.Context(), rows)

	if err != nil || !reflect.DeepEqual(got, rows) || gw.before != nil {
		t.Errorf("err = %v, prepared = %+v, balances asked %v", err, got, gw.before)
	}
}

func TestAYearWithNothingToCarryDeclaresNothing(t *testing.T) {
	got, err := NewWriter(&fakeFile{years: []int{2027}}, &fakeBalances{}).Prepare(t.Context(), rows)

	want := domain.Changeset{Edits: []domain.Edit{domain.AddTransactions{}, domain.SetCarry{Year: 2027}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("err = %v, prepared = %+v, want %+v", err, got, want)
	}
}

func TestPrepareFailuresAreReported(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name string
		file *fakeFile
		gw   *fakeBalances
	}{
		{"the stale years are unknown", &fakeFile{yearsErr: boom}, &fakeBalances{}},
		{"the journal cannot be rendered", &fakeFile{years: []int{2027}, renderErr: boom}, &fakeBalances{}},
		{"hledger cannot add up", &fakeFile{years: []int{2027}}, &fakeBalances{err: boom}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewWriter(test.file, test.gw).Prepare(t.Context(), rows); !errors.Is(err, boom) {
				t.Errorf("Prepare = %v", err)
			}
		})
	}
}

// Apply writes the prepared changeset as it is: the carry is not counted again.
func TestApplyWritesThePreparedChangesetAsItIs(t *testing.T) {
	file := &fakeFile{years: []int{2027}}
	gw := &fakeBalances{}
	prepared := carries(domain.SetCarry{Year: 2027, Balances: checking(900)})

	err := NewWriter(file, gw).Apply(prepared)

	if err != nil || !reflect.DeepEqual(file.applied, []domain.Changeset{prepared}) || gw.before != nil {
		t.Errorf("err = %v, applied = %+v, balances asked %v", err, file.applied, gw.before)
	}
}

func TestApplyReportsTheJournalsOwnFailure(t *testing.T) {
	boom := errors.New("boom")

	if err := NewWriter(&fakeFile{applyErr: boom}, &fakeBalances{}).Apply(rows); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
