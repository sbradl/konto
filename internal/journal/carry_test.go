package journal

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

const carriedYear = `2026-01-01 opening balance  ; konto:carry
    assets:checking  10,00 EUR
    equity:opening-balances

2026-03-01 A
    expenses:x  1,00 EUR
    assets:checking
`

// inJanuary2027 is a ledger whose window is 2026 and 2027, with the given year files.
func inJanuary2027(t *testing.T, years map[int]string) File {
	t.Helper()
	main := journalWith(t, "decimal-mark ,\n").path
	file := NewFile(main, func() time.Time { return time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC) })
	for year, text := range years {
		if err := os.WriteFile(file.yearPath(year), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return file
}

func balances(mantissa int64) []domain.Account {
	return []domain.Account{{Name: "assets:checking", Balance: euro(mantissa)}}
}

func euro(mantissa int64) domain.MultiAmount {
	style := domain.AmountStyle{Spaced: true, DecimalMark: ",", GroupMark: ".", GroupSize: 3}
	return domain.MultiAmount{{Commodity: "EUR", Mantissa: mantissa, Places: 2, Style: style}}
}

func TestSetCarryClosesTheYearBeforeAndOpensTheYear(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: "2027-01-10 B\n    expenses:x  1,00 EUR\n    assets:checking\n"})

	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	wantPrior := carriedYear + "\n2026-12-31 closing balance  ; konto:carry\n    assets:checking  -9,00 EUR\n    equity:opening-balances\n"
	if got := readFile(t, file.yearPath(2026)); got != wantPrior {
		t.Errorf("2026 =\n%s\nwant\n%s", got, wantPrior)
	}
	wantYear := "2027-01-01 opening balance  ; konto:carry\n    assets:checking  9,00 EUR\n    equity:opening-balances\n\n" +
		"2027-01-10 B\n    expenses:x  1,00 EUR\n    assets:checking\n"
	if got := readFile(t, file.yearPath(2027)); got != wantYear {
		t.Errorf("2027 =\n%s\nwant\n%s", got, wantYear)
	}
}

func TestSetCarryReportsAPriorYearFileThatCannotBeRead(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2025: carriedYear, 2026: carriedYear})
	if err := os.Chmod(file.yearPath(2025), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file.yearPath(2025), 0o644) })

	if err := file.Apply(domain.Changeset{Edits: []domain.Edit{domain.SetCarry{Year: 2026}}}); err == nil {
		t.Error("expected an error for the closing year's file that cannot be read")
	}
}

func TestSetCarryReplacesTheCarryItWroteBeforeAndKeepsTheYearsOwnOpening(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: ""})
	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(1200)})

	wantPrior := carriedYear + "\n2026-12-31 closing balance  ; konto:carry\n    assets:checking  -12,00 EUR\n    equity:opening-balances\n"
	if got := readFile(t, file.yearPath(2026)); got != wantPrior {
		t.Errorf("2026 =\n%s\nwant\n%s", got, wantPrior)
	}
	wantYear := "2027-01-01 opening balance  ; konto:carry\n    assets:checking  12,00 EUR\n    equity:opening-balances\n"
	if got := readFile(t, file.yearPath(2027)); got != wantYear {
		t.Errorf("2027 =\n%s\nwant\n%s", got, wantYear)
	}
}

func TestSetCarryWithoutBalancesRemovesTheCarryAndOnlyIt(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: "2027-01-10 B\n    expenses:x  1,00 EUR\n    assets:checking\n"})
	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	apply(t, file, domain.SetCarry{Year: 2027})

	if got := readFile(t, file.yearPath(2026)); got != carriedYear {
		t.Errorf("2026 =\n%s\nwant\n%s", got, carriedYear)
	}
	if got, want := readFile(t, file.yearPath(2027)), "2027-01-10 B\n    expenses:x  1,00 EUR\n    assets:checking\n"; got != want {
		t.Errorf("2027 =\n%s\nwant\n%s", got, want)
	}
}

func TestSetCarryLeavesAYearWithoutALoadedPriorFileAlone(t *testing.T) {
	file := inJanuary2027(t, nil)

	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	for _, year := range []int{2026, 2027} {
		if _, err := os.Stat(file.yearPath(year)); !os.IsNotExist(err) {
			t.Errorf("%d.journal exists: %v", year, err)
		}
	}
}

func TestCarryYearsAreTheYearsAfterTheEarliestWrittenOneAndANewFile(t *testing.T) {
	later := domain.AddTransactions{Transactions: []domain.NewTransaction{{Date: time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC), Description: "x"}}}
	late := domain.AddTransactions{Transactions: []domain.NewTransaction{{Date: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Description: "x"}}}
	early := domain.AddTransactions{Transactions: []domain.NewTransaction{{Date: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), Description: "x"}}}
	tests := []struct {
		name  string
		files map[int]string
		edit  domain.Edit
		want  []int
	}{
		{"a crossing into a new year", map[int]string{2026: carriedYear}, later, []int{2027}},
		{"a late row of last year", map[int]string{2026: carriedYear, 2027: "x\n"}, late, []int{2027}},
		{"a row of the newest year", map[int]string{2026: carriedYear, 2027: "x\n"}, later, nil},
		{"the first year file", nil, later, nil},
		{"a row older than the window", map[int]string{2026: carriedYear, 2027: "x\n"}, early, []int{2026, 2027}},
		{"a row older than the window, a file in between", map[int]string{2024: "x\n", 2025: "x\n", 2027: "x\n"}, early, []int{2025, 2027}},
		{"a new year file with an unloaded year before it", map[int]string{2024: "x\n"}, later, []int{2027}},
		{"no transactions", map[int]string{2026: carriedYear}, domain.Declare{Directive: "payee", Name: "x"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := inJanuary2027(t, test.files)

			got, err := file.CarryYears(domain.Changeset{Edits: []domain.Edit{test.edit}})

			if err != nil || !slices.Equal(got, test.want) {
				t.Errorf("years = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestCarryYearsReportsAnEditThatDoesNotApply(t *testing.T) {
	file := inJanuary2027(t, nil)
	future := domain.AddTransactions{Transactions: []domain.NewTransaction{{Date: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)}}}

	if _, err := file.CarryYears(domain.Changeset{Edits: []domain.Edit{future}}); err == nil {
		t.Error("want the refusal of a row in the future")
	}
}

func TestSetCarryRefusesAYearInTheFuture(t *testing.T) {
	file := inJanuary2027(t, nil)

	if err := file.Apply(domain.Changeset{Edits: []domain.Edit{domain.SetCarry{Year: 2030}}}); err == nil {
		t.Error("want the refusal of a year in the future")
	}
}

func TestSetCarryClosesAnOlderYearFileThatIsNotLoaded(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2024: "2024-02-01 A\n    expenses:x  1,00 EUR\n    assets:checking\n", 2027: ""})

	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	want := "2024-02-01 A\n    expenses:x  1,00 EUR\n    assets:checking\n\n" +
		"2024-12-31 closing balance  ; konto:carry\n    assets:checking  -9,00 EUR\n    equity:opening-balances\n"
	if got := readFile(t, file.yearPath(2024)); got != want {
		t.Errorf("2024 =\n%s\nwant\n%s", got, want)
	}
}

func TestSetCarryWithoutBalancesEmptiesAYearFileThatHeldOnlyTheCarry(t *testing.T) {
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: ""})
	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(900)})

	apply(t, file, domain.SetCarry{Year: 2027})

	if got := readFile(t, file.yearPath(2027)); got != "" {
		t.Errorf("2027 = %q, want it empty", got)
	}
}

func TestSetCarryRemovesEveryCarryOfTheDay(t *testing.T) {
	twice := "2027-01-01 opening balance  ; konto:carry\n    assets:checking  9,00 EUR\n    equity:opening-balances\n\n" +
		"2027-01-01 opening balance  ; konto:carry\n    assets:checking  9,00 EUR\n    equity:opening-balances\n"
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: twice})

	apply(t, file, domain.SetCarry{Year: 2027, Balances: balances(1200)})

	want := "2027-01-01 opening balance  ; konto:carry\n    assets:checking  12,00 EUR\n    equity:opening-balances\n"
	if got := readFile(t, file.yearPath(2027)); got != want {
		t.Errorf("2027 =\n%s\nwant\n%s", got, want)
	}
}

func TestSetCarryWithoutBalancesEmptiesAYearFileWhoseCarryHasNoFinalNewline(t *testing.T) {
	bare := "2027-01-01 opening balance  ; konto:carry\n    assets:checking  9,00 EUR\n    equity:opening-balances"
	file := inJanuary2027(t, map[int]string{2026: carriedYear, 2027: bare})

	apply(t, file, domain.SetCarry{Year: 2027})

	if got := readFile(t, file.yearPath(2027)); got != "" {
		t.Errorf("2027 = %q, want it empty", got)
	}
}
