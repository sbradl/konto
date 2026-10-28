package csvimport

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

// weekImport is three new rows against a checking account that stands at 13.240,00 EUR: one by an old rule,
// one by a rule staged with a new category, one chosen once.
func weekImport() Staged {
	staged := stagedRows(
		Row{
			Date: day("2026-09-30"), Description: "REWE Markt", Payee: "REWE Markt", Purpose: "Einkauf",
			Amount: eur(-4217), Target: "expenses:groceries", RuleID: "2",
		},
		Row{
			Date: day("2026-09-28"), Description: "Kaffeehaus", Payee: "Kaffeehaus", Purpose: "Kaffee",
			Amount: eur(-320), Target: "expenses:cafe", RuleID: "7",
		},
		Row{
			Date: day("2026-09-26"), Description: "Kiosk", Payee: "Kiosk", Purpose: "Zeitung",
			Amount: eur(-200), Target: "expenses:books", Once: true,
		},
	)
	staged.Account.Balance = eur(1324000)
	staged.Closing = "13.192,63"
	staged.NewAccounts = []string{"expenses:cafe", "expenses:unused"}
	staged.Rules = append(staged.Rules, Rule{
		ID: "7", Category: "expenses:cafe", Alternatives: [][]Condition{{{Field: "payee", Pattern: "Kaffeehaus"}}}, Staged: true,
	})
	return staged
}

func planned(t *testing.T, staged Staged) Plan {
	t.Helper()
	plan, err := staged.Plan()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return plan
}

func TestPlanRefusesAnImportWithNoRows(t *testing.T) {
	_, err := Staged{}.Plan()

	if !errors.Is(err, ErrNothingToWrite) {
		t.Errorf("err = %v, want ErrNothingToWrite", err)
	}
}

func TestPlanRefusesANewAccountWithoutAName(t *testing.T) {
	staged := stagedRows(debit("A"))
	staged.Rows[0].Target, staged.NewAccount = "expenses:x", true

	_, err := staged.Plan()

	if !errors.Is(err, errUnnamedAccount) {
		t.Errorf("err = %v, want the account to need a name", err)
	}
}

func TestPlanRefusesWhileRowsNeedACategory(t *testing.T) {
	for want, rows := range map[string][]Row{
		"1 row needs a category": {debit("A")},
		"2 rows need a category": {debit("A"), debit("B")},
	} {
		_, err := stagedRows(rows...).Plan()

		if got := refusal(t, err); got != want {
			t.Errorf("refusal = %q, want %q", got, want)
		}
	}
}

func TestPlanWritesRowsOldestFirstWithTheAssertionOnTheNewest(t *testing.T) {
	plan := planned(t, weekImport())

	var added domain.AddTransactions
	for _, edit := range plan.Changeset.Edits {
		if e, ok := edit.(domain.AddTransactions); ok {
			added = e
		}
	}
	var lines []string
	for _, transaction := range added.Transactions {
		lines = append(lines, transaction.Lines()...)
	}
	want := []string{
		"2026-09-26 Kiosk  ; Zeitung",
		"    expenses:books  2,00 EUR",
		"    assets:checking",
		"2026-09-28 Kaffeehaus  ; Kaffee",
		"    expenses:cafe  3,20 EUR  ; konto-rule:7",
		"    assets:checking",
		"2026-09-30 REWE Markt  ; Einkauf",
		"    expenses:groceries  42,17 EUR  ; konto-rule:2",
		"    assets:checking  -42,17 EUR = 13.192,63 EUR",
	}
	if !slices.Equal(lines, want) {
		t.Errorf("transactions =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	if plan.Count != 3 || plan.Account != checking {
		t.Errorf("count, account = %d, %q", plan.Count, plan.Account)
	}
}

func TestPlanDeclaresWhatIsNewAndWritesTheStagedRules(t *testing.T) {
	plan := planned(t, weekImport())

	want := []domain.Edit{
		domain.Declare{Directive: "account", Name: "expenses:cafe"},
		domain.Declare{Directive: "payee", Name: "REWE Markt"},
		domain.Declare{Directive: "payee", Name: "Kaffeehaus"},
		domain.Declare{Directive: "payee", Name: "Kiosk"},
		domain.Declare{Directive: "tag", Name: "konto-rule"},
	}
	if got := plan.Changeset.Edits[:len(want)]; !reflect.DeepEqual(got, want) {
		t.Errorf("declarations = %+v, want %+v", got, want)
	}
	last := plan.Changeset.Edits[len(plan.Changeset.Edits)-1]
	if rule, ok := last.(domain.SetCategoryRule); !ok || rule.ID != "7" ||
		rule.Text != "if %payee Kaffeehaus\n    account2 expenses:cafe\n    comment2 konto-rule:7" {
		t.Errorf("last edit = %+v, want rule 7", last)
	}
}

func TestPlanDeclaresTheRuleTagForARuleWithoutRows(t *testing.T) {
	staged := weekImport()
	staged.Rows[0].RuleID, staged.Rows[1].RuleID = "", ""

	plan := planned(t, staged)

	if !slices.Contains(plan.Changeset.Edits, domain.Edit(domain.Declare{Directive: "tag", Name: "konto-rule"})) {
		t.Error("the staged rule's tag is not declared")
	}
}

func TestPlanDeclaresNoRuleTagWhenNothingCarriesOne(t *testing.T) {
	staged := weekImport()
	staged.Rows[0].RuleID, staged.Rows[1].RuleID = "", ""
	staged.Rules = nil

	plan := planned(t, staged)

	if slices.Contains(plan.Changeset.Edits, domain.Edit(domain.Declare{Directive: "tag", Name: "konto-rule"})) {
		t.Error("a rule tag is declared though no posting carries one")
	}
}

func TestPlanPreviewListsTransactionsNewestFirstThenDirectivesAndRules(t *testing.T) {
	plan := planned(t, weekImport())

	if plan.Preview[0] != "2026-09-30 REWE Markt  ; Einkauf" {
		t.Errorf("preview starts %q, want the newest transaction", plan.Preview[0])
	}
	text := strings.Join(plan.Preview, "\n")
	for _, want := range []string{
		"2026-09-26 Kiosk", "account expenses:cafe", "payee Kaffeehaus", "tag konto-rule", "comment2 konto-rule:7",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview lacks %q:\n%s", want, text)
		}
	}
}

func TestPlanBlocksAnExportThatDoesNotReachTheBanksBalance(t *testing.T) {
	staged := weekImport()
	staged.Closing = "13.182,03"

	_, err := staged.Plan()

	var blocked BalanceMismatchError
	if !errors.As(err, &blocked) {
		t.Fatalf("err = %v, want a BalanceMismatchError", err)
	}
	if blocked.Bank.Format() != "13.182,03 EUR" || blocked.Journal.Format() != "13.192,63 EUR" ||
		blocked.Difference.Format() != "-10,60 EUR" {
		t.Errorf("mismatch = %+v", blocked)
	}
	want := "write blocked — bank 13.182,03 EUR · journal 13.192,63 EUR · difference -10,60 EUR"
	if err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestPlanReportsAnUnreadableClosingBalance(t *testing.T) {
	for _, closing := range []string{"", "x", "1,234", "1.2x,00"} {
		staged := weekImport()
		staged.Closing = closing

		if _, err := staged.Plan(); err == nil || !strings.Contains(err.Error(), "closing balance") {
			t.Errorf("closing %q: err = %v, want a closing-balance error", closing, err)
		}
	}
}

func TestParseAmountReadsGermanNumbersInTheStyleOfTheLedger(t *testing.T) {
	like := eur(0)[0]
	for text, want := range map[string]int64{"13.182,03": 1318203, "-5,5": -550, "7": 700, "0,01": 1} {
		got, err := postbank().amount(like, text)

		if err != nil || got.Mantissa != want || got.Format() != euroFormat(want) {
			t.Errorf("amount(%q) = %v, %v; want mantissa %d", text, got, err, want)
		}
	}
}

func euroFormat(mantissa int64) string { return eur(mantissa)[0].Format() }

func TestPlanForANewAccountOpensItsBalanceTheDayBeforeTheOldestRow(t *testing.T) {
	staged := Staged{
		NewAccount: true, Named: "assets:holiday", IBAN: "DE27100777770209299700", Format: FormatPostbank,
		Zero: eur(0), Closing: "4.412,34",
		Rows: []Row{
			{Date: day("2026-09-15"), Description: "Kontoabrechnung", Purpose: "Zinsen", Amount: eur(1234), Target: "income:interest", RuleID: "6"},
			{Date: day("2026-09-01"), Description: "Max", Purpose: "Umbuchung", Amount: eur(50000), Target: "assets:transfers", RuleID: "7"},
			{Date: day("2026-08-20"), Description: "Kontoabrechnung", Purpose: "Geldautomat", Amount: eur(-10000), Target: "expenses:cash", RuleID: "8"},
		},
		NewAccounts: []string{"income:interest", "expenses:cash"},
	}

	plan := planned(t, staged)

	text := strings.Join(plan.Preview, "\n")
	for _, want := range []string{
		"2026-08-19 opening balance\n    assets:holiday  4.000,00 EUR = 4.000,00 EUR\n    equity:opening-balances",
		"2026-09-15 Kontoabrechnung  ; Zinsen\n    income:interest  -12,34 EUR  ; konto-rule:6\n    assets:holiday  12,34 EUR = 4.412,34 EUR",
		"account assets:holiday  ; konto:source:import, konto:iban:DE27100777770209299700, konto:format:postbank",
		"payee opening balance", "account income:interest", "account expenses:cash",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview lacks %q:\n%s", want, text)
		}
	}
	if plan.Count != 3 || plan.Account != "assets:holiday" {
		t.Errorf("count, account = %d, %q", plan.Count, plan.Account)
	}
}

func TestWithNameChecksTheLeafAndTheKindOfAccount(t *testing.T) {
	staged := stagedRows()
	staged.NewAccounts = []string{"expenses:cafe"}
	for name, want := range map[string]string{
		"":                     "the account needs a name",
		"expenses:holiday":     "must be an assets: or liabilities:",
		"assets:checking":      "checking",
		"assets:giro:checking": "checking",
		"assets:cafe":          "cafe",
	} {
		_, err := staged.WithName(name)

		if got := refusal(t, err); !strings.Contains(got, want) {
			t.Errorf("WithName(%q) refused with %q, want it to mention %q", name, got, want)
		}
	}
}

func TestWithNameStagesAFreshAccountName(t *testing.T) {
	staged := stagedRows()
	staged.NewAccount = true

	named, err := staged.WithName("assets:holiday")

	if err != nil || named.AccountName() != "assets:holiday" || staged.Named != "" {
		t.Errorf("named = %q, %v (original %q)", named.AccountName(), err, staged.Named)
	}
}

func TestSuggestAccountNameEndsInTheLastDigitsOfTheIBAN(t *testing.T) {
	if got := SuggestAccountName("DE27100777770209299700"); got != "assets:account-9700" {
		t.Errorf("name = %q", got)
	}
	if got := SuggestAccountName("12"); got != "assets:account-12" {
		t.Errorf("short name = %q", got)
	}
}

func TestOpenRemembersThePathOfTheExport(t *testing.T) {
	staged, _, err := open(t, "DE89370400440532013000", ledgerWith(importAccount()))

	if err != nil || staged.Path != "/dl/sept.csv" {
		t.Errorf("path = %q, err = %v", staged.Path, err)
	}
}

// reread is the export opened again after the journal changed: what the disk's rules make of it.
func reread() Staged {
	fresh := weekImport()
	fresh.Rules = ParseRules(rulesText)
	fresh.NewAccounts = nil
	fresh.Rows = []Row{
		{Date: day("2026-09-30"), Description: "REWE Markt", Payee: "REWE Markt", Purpose: "Einkauf", Amount: eur(-4217), Target: "expenses:groceries", RuleID: "2"},
		{Date: day("2026-09-28"), Description: "Kaffeehaus", Payee: "Kaffeehaus", Purpose: "Kaffee", Amount: eur(-320)},
		{Date: day("2026-09-26"), Description: "Kiosk", Payee: "Kiosk", Purpose: "Zeitung", Amount: eur(-200)},
	}
	return fresh
}

func TestRestageKeepsTheStagedRulesNewCategoriesAndChoicesOfTheEarlierImport(t *testing.T) {
	restaged := reread().Restage(weekImport())

	want := weekImport().Rows
	if !reflect.DeepEqual(restaged.Rows, want) {
		t.Errorf("rows =\n%+v\nwant\n%+v", restaged.Rows, want)
	}
	if rules := restaged.StagedRules(); len(rules) != 1 || rules[0].ID != "7" {
		t.Errorf("staged rules = %+v, want rule 7", rules)
	}
	if !slices.Equal(restaged.NewAccounts, []string{"expenses:cafe", "expenses:unused"}) {
		t.Errorf("new accounts = %v", restaged.NewAccounts)
	}
}

func TestRestageKeepsAChoiceMadeOnceEvenOverARule(t *testing.T) {
	earlier := weekImport()
	earlier.Rows[0].Target, earlier.Rows[0].RuleID, earlier.Rows[0].Once = "expenses:books", "", true

	restaged := reread().Restage(earlier)

	if row := restaged.Rows[0]; row.Target != "expenses:books" || row.RuleID != "" || !row.Once {
		t.Errorf("row = %+v, want the once choice", row)
	}
}

func TestRestageReplacesAWidenedRuleAndLeavesNewlyMatchedRowsToIt(t *testing.T) {
	earlier := weekImport()
	earlier.Rules = ParseRules(rulesText)
	widened := earlier.Rules[0]
	widened.Alternatives = append(widened.Alternatives, []Condition{{Field: "payee", Pattern: "Kaffeehaus"}})
	widened.Staged = true
	earlier.Rules[0] = widened

	restaged := reread().Restage(earlier)

	if row := restaged.Rows[1]; row.Target != widened.Category || row.RuleID != widened.ID {
		t.Errorf("row = %+v, want the widened rule's category", row)
	}
	if !reflect.DeepEqual(restaged.Rules[0], widened) {
		t.Errorf("rule = %+v, want the widened one", restaged.Rules[0])
	}
}

func TestRestageKeepsTheNameOfANewAccount(t *testing.T) {
	earlier := weekImport()
	earlier.Named = "assets:holiday"
	fresh := reread()
	fresh.NewAccount = true

	if got := fresh.Restage(earlier).Named; got != "assets:holiday" {
		t.Errorf("named = %q", got)
	}
}

func TestRestageDoesNotMatchTwoRowsToOneChoice(t *testing.T) {
	earlier := reread()
	earlier.Rows = []Row{{Date: day("2026-09-26"), Description: "Kiosk", Purpose: "Zeitung", Amount: eur(-200), Target: "expenses:books", Once: true}}
	fresh := reread()
	fresh.Rows = []Row{fresh.Rows[2], fresh.Rows[2]}

	restaged := fresh.Restage(earlier)

	if !restaged.Rows[0].Once || restaged.Rows[1].Once {
		t.Errorf("rows = %+v, want only the first to take the choice", restaged.Rows)
	}
}

func TestPlanWritesEurosWhenTheLedgerHasNoBalanceToTakeAStyleFrom(t *testing.T) {
	staged := weekImport()
	staged.Zero = nil

	plan := planned(t, staged)

	if text := strings.Join(plan.Preview, "\n"); !strings.Contains(text, "assets:checking  -42,17 EUR = 13.192,63 EUR") {
		t.Errorf("preview =\n%s", text)
	}
}
