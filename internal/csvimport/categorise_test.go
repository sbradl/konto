package csvimport

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

const rulesText = `if %payee Vermieter Mueller
    account2 expenses:rent
    comment2 konto-rule:1

if %payee REWE
    account2 expenses:groceries
    comment2 konto-rule:2

if %payee Sparen
    account2 assets:transfers
    comment2 konto-rule:5
`

func debit(payee string) Row {
	return Row{Description: payee, Payee: payee, Type: "Kartenzahlung", Purpose: "Zeitung", Amount: eur(-200)}
}

func stagedRows(rows ...Row) Staged {
	return Staged{
		Rows: rows, Rules: ParseRules(rulesText), Zero: eur(0),
		Account: domain.Account{Name: checking, Source: domain.SourceImport},
		Known: []domain.Account{
			{Name: checking, Source: domain.SourceImport},
			{Name: "assets:savings", Source: domain.SourceImport},
			{Name: "assets:cash", Source: domain.SourceManual},
			{Name: "assets:transfers", Source: domain.SourceClearing},
			{Name: "assets:old", Source: domain.SourceImport, Closed: day("2026-03-01")},
			{Name: "equity:opening-balances"},
			{Name: "expenses:rent"},
			{Name: "expenses:groceries"},
			{Name: "income:salary"},
			{Name: "expenses:old", Closed: day("2026-03-01")},
		},
	}
}

func refusal(t *testing.T, err error) string {
	t.Helper()
	var refused domain.RefusalError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	return refused.Reason
}

func TestParseRulesReadsTheIdCategoryAndAlternatives(t *testing.T) {
	text := "if %payee A\n%type B\n& %purpose C\n    account2 expenses:x\n    comment2 konto-rule:7\n"
	got := ParseRules(text)
	want := []Rule{{ID: "7", Category: "expenses:x", Alternatives: [][]Condition{
		{{"payee", "A"}}, {{"type", "B"}, {"purpose", "C"}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rules = %+v, want %+v", got, want)
	}
}

func TestParseRulesLeavesOutBlocksWithoutAnIDOrACategory(t *testing.T) {
	text := "if %payee A\n    account2 expenses:x\n\nif %payee B\n    comment2 konto-rule:3\n\n; note\nif %payee C\r\n" +
		"    account2 expenses:c\r\n    comment2 konto-rule:4\r\n"
	got := ParseRules(text)
	if len(got) != 1 || got[0].ID != "4" || got[0].Category != "expenses:c" {
		t.Fatalf("rules = %+v, want only rule 4", got)
	}
}

func TestParseRulesIgnoresIndentedLinesOutsideABlockAndUnknownDirectives(t *testing.T) {
	text := "    account2 expenses:lost\nif %payee A\n    skip\n    account2 expenses:a\n    comment2 konto-rule:1\n"
	if got := ParseRules(text); len(got) != 1 || got[0].Category != "expenses:a" {
		t.Fatalf("rules = %+v", got)
	}
}

func TestRuleTextWritesIfOrAndLinesAndTheRuleTag(t *testing.T) {
	rule := Rule{ID: "2", Category: "expenses:groceries", Alternatives: [][]Condition{
		{{"payee", "REWE"}}, {{"payee", "Bäckerei"}, {"purpose", "Brot"}},
	}}
	want := "if %payee REWE\n%payee Bäckerei\n& %purpose Brot\n    account2 expenses:groceries\n    comment2 konto-rule:2"
	if got := rule.Text(); got != want {
		t.Fatalf("text =\n%s\nwant\n%s", got, want)
	}
}

func TestRuleMatchesBySubstringIgnoringCaseAndNeedsEveryConditionOfAnAlternative(t *testing.T) {
	rule := Rule{Alternatives: [][]Condition{{{"payee", "rewe"}}, {{"payee", "Kiosk"}, {"purpose", "Zeit"}}}}
	for payee, want := range map[string]bool{"REWE Markt": true, "Kiosk": true, "Bäcker": false} {
		if got := rule.Matches(debit(payee)); got != want {
			t.Errorf("Matches(%q) = %v, want %v", payee, got, want)
		}
	}
	other := debit("Kiosk")
	other.Purpose = "Tabak"
	if rule.Matches(other) {
		t.Error("the purpose condition did not hold but the rule matched")
	}
}

func TestRuleWithAnInvalidPatternOrUnknownFieldMatchesNothing(t *testing.T) {
	for _, condition := range []Condition{{"payee", "("}, {"colour", "x"}} {
		if (Rule{Alternatives: [][]Condition{{condition}}}).Matches(debit("x")) {
			t.Errorf("%+v matched", condition)
		}
	}
}

func TestRuleMatchesTheTypeAndPurposeColumns(t *testing.T) {
	row := Row{Type: "Auszahlung", Purpose: "Geldautomat"}
	if !(Rule{Alternatives: [][]Condition{{{"type", "Ausz"}, {"purpose", "automat"}}}}).Matches(row) {
		t.Error("type and purpose did not match")
	}
}

func TestMatchFieldIsThePayeeElseTheType(t *testing.T) {
	if debit("Kiosk").MatchField() != "payee" || (Row{Type: "Auszahlung"}).MatchField() != "type" {
		t.Error("wrong match field")
	}
}

func TestIsTransferIsAPostingToAnAssetOrLiability(t *testing.T) {
	for target, want := range map[string]bool{
		"assets:cash": true, "liabilities:card": true, "expenses:x": false, "": false,
	} {
		if got := (Row{Target: target}).IsTransfer(); got != want {
			t.Errorf("IsTransfer(%q) = %v", target, got)
		}
	}
}

func names(candidates []Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.Name)
	}
	return out
}

func TestCandidatesListTheDirectionsCategoriesFirstThenTheOtherThenTransfers(t *testing.T) {
	s := stagedRows(debit("Kiosk"))
	s.NewAccounts = []string{"expenses:coffee", "equity:x"}
	want := []string{
		"expenses:coffee", "expenses:groceries", "expenses:rent", "income:salary", "assets:cash", "assets:savings",
	}
	if got := names(s.Candidates(s.Rows[0])); !reflect.DeepEqual(got, want) {
		t.Fatalf("outflow candidates = %v, want %v", got, want)
	}
	inflow := Row{Amount: eur(100)}
	want = []string{"income:salary", "expenses:coffee", "expenses:groceries", "expenses:rent", "assets:cash", "assets:savings"}
	if got := names(s.Candidates(inflow)); !reflect.DeepEqual(got, want) {
		t.Fatalf("inflow candidates = %v, want %v", got, want)
	}
}

func TestAnImportFedTransferTargetIsPostedToTheClearingAccount(t *testing.T) {
	s := stagedRows(debit("Kiosk"))
	var savings, cash Candidate
	for _, c := range s.Candidates(s.Rows[0]) {
		switch c.Name {
		case "assets:savings":
			savings = c
		case "assets:cash":
			cash = c
		}
	}
	if savings.Target != "assets:transfers" || !savings.Transfer {
		t.Errorf("savings = %+v", savings)
	}
	if cash.Target != "assets:cash" || !cash.Transfer {
		t.Errorf("cash = %+v", cash)
	}
}

func TestTheClearingAccountIsTheOneTheLedgerNamesElseTheDefault(t *testing.T) {
	s := stagedRows(debit("x"))
	s.Known = []domain.Account{{Name: "assets:bridge", Source: domain.SourceClearing}, {Name: "assets:s", Source: domain.SourceImport}}
	if got := s.Candidates(s.Rows[0])[0].Target; got != "assets:bridge" {
		t.Errorf("target = %q", got)
	}
	s.Known = []domain.Account{{Name: "assets:s", Source: domain.SourceImport}}
	if got := s.Candidates(s.Rows[0])[0].Target; got != ClearingAccount {
		t.Errorf("target = %q", got)
	}
}

func TestMatchingFiltersByCaseInsensitiveSubstring(t *testing.T) {
	got := Matching([]Candidate{{Name: "expenses:Groceries"}, {Name: "expenses:rent"}}, "gro")
	if len(got) != 1 || got[0].Name != "expenses:Groceries" {
		t.Fatalf("matching = %v", got)
	}
}

func TestSuggestNameUsesTheDirectionPrefixUnlessTheNameHasOne(t *testing.T) {
	out, in := Row{Amount: eur(-1)}, Row{Amount: eur(1)}
	for _, c := range []struct {
		row         Row
		typed, want string
	}{
		{out, "coffee", "expenses:coffee"},
		{in, "gifts", "income:gifts"},
		{out, "income:x", "income:x"},
		{out, "  ", ""},
		{out, " tea ", "expenses:tea"},
	} {
		if got := SuggestName(c.row, c.typed); got != c.want {
			t.Errorf("SuggestName(%q) = %q, want %q", c.typed, got, c.want)
		}
	}
}

func TestCreateStagesTheDirectiveAndOffersTheNameLater(t *testing.T) {
	s := stagedRows(debit("Kiosk"))
	next, candidate, err := s.Create("expenses:coffee")
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Target != "expenses:coffee" || candidate.Transfer {
		t.Errorf("candidate = %+v", candidate)
	}
	if !reflect.DeepEqual(next.NewAccounts, []string{"expenses:coffee"}) || len(s.NewAccounts) != 0 {
		t.Errorf("new accounts: staged %v, original %v", next.NewAccounts, s.NewAccounts)
	}
	if !strings.Contains(strings.Join(names(next.Candidates(s.Rows[0])), " "), "expenses:coffee") {
		t.Error("the created category is not offered")
	}
}

func TestCreateRefusesALeafSomeAccountHasAndAnEmptyName(t *testing.T) {
	s := stagedRows(debit("Kiosk"))
	for name, mention := range map[string]string{
		"expenses:food:groceries": "groceries", "income:rent": "rent", "income:salary": "salary", "": "needs a name",
	} {
		_, _, err := s.Create(name)
		if reason := refusal(t, err); !strings.Contains(reason, mention) {
			t.Errorf("Create(%q): %q lacks %q", name, reason, mention)
		}
	}
	next, _, _ := s.Create("expenses:x")
	if _, _, err := next.Create("income:x"); err == nil {
		t.Error("a staged name did not block its leaf")
	}
}

func TestOnceCategorisesOneRowWithoutARule(t *testing.T) {
	s := stagedRows(debit("REWE"), debit("Kiosk"))
	s.Rows[0].Target, s.Rows[0].RuleID = "expenses:groceries", "2"
	next := s.Once(0, Candidate{Name: "expenses:books", Target: "expenses:books"})
	if got := next.Rows[0]; got.Target != "expenses:books" || got.RuleID != "" || !got.Once {
		t.Errorf("row = %+v", got)
	}
	if next.Rows[1].Target != "" || s.Rows[0].Target != "expenses:groceries" {
		t.Error("another row or the original changed")
	}
}

func TestNextRuleIDContinuesAfterTheHighestNumericID(t *testing.T) {
	s := stagedRows()
	if got := s.NextRuleID(); got != "6" {
		t.Errorf("next id = %q", got)
	}
	s.Rules = append(s.Rules, Rule{ID: "legacy"})
	if got := s.NextRuleID(); got != "6" {
		t.Errorf("next id with a non-numeric id = %q", got)
	}
	if got := (Staged{}).NextRuleID(); got != "1" {
		t.Errorf("first id = %q", got)
	}
}

func TestDraftForEscapesTheDescriptionAndTestsTheTypeWithoutAPayee(t *testing.T) {
	payee := Row{Payee: "Café (Nord) & Söhne.", Type: "Kartenzahlung"}
	cash := Row{Type: "Auszahlung"}
	s := stagedRows(payee, cash)
	if got := s.DraftFor(0); got != (Draft{Field: "payee", Pattern: `Café \(Nord\) & Söhne\.`}) {
		t.Errorf("draft = %+v", got)
	}
	if got := s.DraftFor(1); got != (Draft{Field: "type", Pattern: "Auszahlung"}) {
		t.Errorf("draft = %+v", got)
	}
}

func TestDraftBecomesARuleWithAnOptionalPurposeCondition(t *testing.T) {
	rule := Draft{Field: "payee", Pattern: "PayPal", Purpose: "Thalia"}.Rule("6", "expenses:books")
	want := "if %payee PayPal\n& %purpose Thalia\n    account2 expenses:books\n    comment2 konto-rule:6"
	if rule.Text() != want || !rule.Staged {
		t.Errorf("rule = %q staged %v", rule.Text(), rule.Staged)
	}
	if got := (Draft{Field: "payee", Pattern: "x"}).Alternative(); len(got) != 1 {
		t.Errorf("alternative = %v", got)
	}
}

func TestImpactCountsMatchesAndWhatItWouldTakeFromOtherRules(t *testing.T) {
	s := stagedRows(debit("Kaffeehaus"), debit("Kaffeehaus Nord"), debit("REWE Markt"), debit("REWE Nord"))
	s.Rows[2].Target, s.Rows[2].RuleID = "expenses:groceries", "2"
	s.Rows[3].Target, s.Rows[3].RuleID = "expenses:groceries", "2"
	rule := Draft{Field: "payee", Pattern: "Kaffeehaus"}.Rule("6", "expenses:cafe")
	if got := s.Impact(rule); got.Matches != 2 || got.Takes != 0 {
		t.Errorf("impact = %+v", got)
	}
	broad := Draft{Field: "payee", Pattern: "e"}.Rule("6", "expenses:cafe")
	got := s.Impact(broad)
	if got.Matches != 4 || got.Takes != 2 || !reflect.DeepEqual(got.From, []string{"2"}) {
		t.Errorf("impact = %+v", got)
	}
}

func TestAddRuleCategorisesEveryUnclaimedRowItMatches(t *testing.T) {
	s := stagedRows(debit("Kaffeehaus"), debit("Kaffeehaus Nord"), debit("Kiosk"))
	s = s.Once(1, Candidate{Target: "expenses:groceries"})
	next, err := s.AddRule(Draft{Field: "payee", Pattern: "Kaffeehaus"}.Rule("6", "expenses:cafe"))
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Rows[0]; got.Target != "expenses:cafe" || got.RuleID != "6" {
		t.Errorf("row 0 = %+v", got)
	}
	if got := next.Rows[1]; got.Target != "expenses:groceries" || got.RuleID != "" {
		t.Errorf("a row chosen once changed: %+v", got)
	}
	if next.Rows[2].Target != "" {
		t.Error("a row the rule does not match was categorised")
	}
	if len(next.StagedRules()) != 1 || len(s.Rules) != 3 {
		t.Errorf("staged rules %d, original rules %d", len(next.StagedRules()), len(s.Rules))
	}
}

func TestAddRuleRefusesWhatItCannotStage(t *testing.T) {
	rewe := debit("REWE Markt")
	rewe.Target, rewe.RuleID = "expenses:groceries", "2"
	s := stagedRows(rewe, debit("Kiosk"))
	for _, c := range []struct {
		draft    Draft
		category string
		mention  string
	}{
		{Draft{Field: "payee", Pattern: "rewe"}, "expenses:x", "rule 2"},
		{Draft{Field: "payee", Pattern: ""}, "expenses:x", "pattern is required"},
		{Draft{Field: "payee", Pattern: "Kiosk", Purpose: ""}, "", "choose a category"},
		{Draft{Field: "payee", Pattern: "Ki", Purpose: "("}, "expenses:x", "not a valid"},
		{Draft{Field: "payee", Pattern: "("}, "expenses:x", "not a valid"},
	} {
		_, err := s.AddRule(c.draft.Rule("6", c.category))
		if reason := refusal(t, err); !strings.Contains(reason, c.mention) {
			t.Errorf("%+v: %q lacks %q", c.draft, reason, c.mention)
		}
	}
}

func TestWidenAddsAnAlternativeAndClaimsItsRows(t *testing.T) {
	rewe := debit("REWE Markt")
	rewe.Target, rewe.RuleID = "expenses:groceries", "2"
	s := stagedRows(rewe, debit("Bäckerei"), debit("Kiosk"))
	alternative := []Condition{{"payee", "Bäckerei"}}
	preview, ok := s.Widened("2", alternative)
	if !ok || !strings.Contains(preview.Text(), "if %payee REWE\n%payee Bäckerei\n") {
		t.Fatalf("preview = %q", preview.Text())
	}
	next, err := s.Widen("2", alternative)
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Rows[1]; got.Target != "expenses:groceries" || got.RuleID != "2" {
		t.Errorf("row = %+v", got)
	}
	if next.Rows[2].Target != "" || len(next.StagedRules()) != 1 || !next.StagedRules()[0].Staged {
		t.Errorf("rows %+v, staged %v", next.Rows, next.StagedRules())
	}
	if len(s.Rules[1].Alternatives) != 1 {
		t.Error("the original rule changed")
	}
}

func TestWidenRefusesAnOverlapAndAnUnknownRule(t *testing.T) {
	sparen := debit("Sparen")
	sparen.Target, sparen.RuleID = "assets:transfers", "5"
	s := stagedRows(sparen)
	_, err := s.Widen("2", []Condition{{"payee", "Spar"}})
	if reason := refusal(t, err); !strings.Contains(reason, "rule 5") {
		t.Errorf("reason = %q", reason)
	}
	_, err = s.Widen("99", []Condition{{"payee", "x"}})
	if reason := refusal(t, err); !strings.Contains(reason, "99") {
		t.Errorf("reason = %q", reason)
	}
	if _, ok := s.Widened("99", nil); ok {
		t.Error("an unknown rule was widened")
	}
}

func TestNetIncomeChangeCountsWhatWasChosenHereAndNotTransfers(t *testing.T) {
	s := stagedRows(debit("a"), debit("b"), debit("c"), debit("d"), debit("e"))
	s.Rules = append(s.Rules, Rule{ID: "6", Staged: true})
	for i, target := range []string{"expenses:x", "income:y", "assets:transfers", "expenses:z", "expenses:old-rule"} {
		s.Rows[i].Target = target
	}
	s.Rows[0].Once = true // -2,00
	s.Rows[1].Once, s.Rows[1].Amount = true, eur(500)
	s.Rows[2].Once = true  // a transfer
	s.Rows[3].RuleID = "6" // a rule staged here: -2,00
	s.Rows[4].RuleID = "2" // a rule already in categories.rules
	if got := s.NetIncomeChange().Format(); got != "1,00 EUR" {
		t.Errorf("net income change = %s", got)
	}
	if got := stagedRows(debit("a")).NetIncomeChange().Format(); got != "0,00 EUR" {
		t.Errorf("net income change = %s", got)
	}
}

func TestOpenReadsTheRulesAndGivesRowsTheirRawTypeAndPayee(t *testing.T) {
	conv := &fakeConverter{transactions: []domain.Transaction{
		converted("2026-09-30", "Kiosk", "Zeitung", -200, "", ""),
		converted("2026-09-29", "Other", "x", -100, "", ""),
	}}
	files := fakeFiles{data: map[string][]byte{"/dl/sept.csv": export("DE89370400440532013000"), "/data/categories.rules": []byte(rulesText)}}
	ledger := ledgerWith(importAccount())
	ledger.Accounts[0].Balance = eur(100)
	staged, err := service(files, conv).Open(context.Background(), "/dl/sept.csv", ledger)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.Rules) != 3 || staged.Rules[1].ID != "2" || len(staged.Known) != 1 || staged.Zero.Format() != "0,00 EUR" {
		t.Errorf("staged = %+v", staged)
	}
	if got := staged.Rows[1]; got.Type != "Kartenzahlung" || got.Payee != "Kiosk" {
		t.Errorf("row = %+v, want its raw fields", got)
	}
	if got := staged.Rows[0]; got.Type != "" || got.Payee != "" {
		t.Errorf("a row with no export line has raw fields: %+v", got)
	}
}

func TestOpenGoesOnWithoutRulesWhenTheRulesFileCannotBeRead(t *testing.T) {
	files := &failingRules{fakeFiles{data: map[string][]byte{"/dl/sept.csv": export("DE89370400440532013000")}}}
	staged, err := service(files.fakeFiles, &fakeConverter{}).Open(context.Background(), "/dl/sept.csv", ledgerWith(importAccount()))
	if err != nil || staged.Rules != nil {
		t.Fatalf("staged rules %v, err %v", staged.Rules, err)
	}
	svc := NewService(files, &fakeConverter{}, "/data/categories.rules")
	if staged, err = svc.Open(context.Background(), "/dl/sept.csv", ledgerWith(importAccount())); err != nil || staged.Rules != nil {
		t.Fatalf("staged rules %v, err %v", staged.Rules, err)
	}
}

type failingRules struct{ fakeFiles }

func (f *failingRules) ReadFile(path string) ([]byte, error) {
	if path == "/data/categories.rules" {
		return nil, errors.New("no such file")
	}
	return f.fakeFiles.ReadFile(path)
}

func TestRecordsSkipLinesThatAreNoBookings(t *testing.T) {
	lines := []string{
		"junk;1;2;3;4", "Buchungstag;Wert", "short;line", "xx.1.2026;;A;B;C", "", "Kontostand;30.9.2026;;;1;EUR",
		"1.9.2026;1.9.2026;Umbuchung;Sparen;Sparen",
	}
	want := []record{{date: "2026-09-01", kind: "Umbuchung", payee: "Sparen", purpose: "Sparen"}}
	if got := recordsOf(lines); !reflect.DeepEqual(got, want) {
		t.Errorf("records = %+v, want %+v", got, want)
	}
}

func TestRawFieldsOfABookingWithoutPayeeCarryTheTypeAsDescription(t *testing.T) {
	rows := withRawFields(
		[]Row{{Date: day("2026-09-29"), Description: "Auszahlung", Purpose: "Geldautomat"}},
		[]record{{date: "2026-09-29", kind: "Auszahlung", purpose: "Geldautomat"}},
	)
	if rows[0].Type != "Auszahlung" || rows[0].Payee != "" {
		t.Errorf("row = %+v", rows[0])
	}
}

func TestWidenRefusesAnInvalidPattern(t *testing.T) {
	_, err := stagedRows(debit("x")).Widen("2", []Condition{{"payee", "("}})
	if reason := refusal(t, err); !strings.Contains(reason, "not a valid") {
		t.Errorf("reason = %q", reason)
	}
}

func TestColumnIsEmptyBeyondTheEndOfALine(t *testing.T) {
	if column([]string{"a"}, 3) != "" || column([]string{"a", "b"}, 1) != "b" {
		t.Error("column misreads a short line")
	}
}
