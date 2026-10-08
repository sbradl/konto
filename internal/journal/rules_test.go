package journal

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sbradl/konto/internal/domain"
)

const rulesJournal = `account assets:checking
~ monthly from 2026-11-01  Top-up
    assets:spare  10,00 EUR
    assets:checking

~ monthly from 2026-01-01 to 2026-03-01  Old top-up
    ; a comment
    (assets:spare)  10,00 EUR
    assets:checking

~ 2026-01-05  Once-off
	[assets:spare]	10,00 EUR
    assets:checking

~ monthly  Elsewhere
    assets:savings  10,00 EUR
    assets:checking

~ weekly
    * assets:spare  1,00 EUR
    assets:checking
2026-01-01 A transaction that mentions the spare account
    assets:spare  1,00 EUR
    assets:checking
~ monthly  Trailing space
`

func TestRulesReferencingNamesEveryRuleThatPostsToTheAccount(t *testing.T) {
	got, err := journalWith(t, rulesJournal).RulesReferencing("assets:spare")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"Top-up", "Old top-up", "Once-off", "weekly"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %v, want %v", got, want)
	}
}

func TestRulesReferencingFindsRulesInIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	main := writeJournal(t, dir, "main.journal", "include rules.journal\n")
	writeJournal(t, dir, "rules.journal", "~ monthly  Included\n    assets:spare  1,00 EUR\n    assets:checking\n")

	got, err := main.RulesReferencing("assets:spare")

	if err != nil || !reflect.DeepEqual(got, []string{"Included"}) {
		t.Errorf("rules = %v, %v; want [Included]", got, err)
	}
}

func TestRulesReferencingReportsAnUnreadableJournal(t *testing.T) {
	if _, err := NewFile("/nonexistent/main.journal", septemberOf(2026)).RulesReferencing("assets:spare"); err == nil {
		t.Error("expected an error")
	}
}

func TestRuleNameFallsBackToThePeriod(t *testing.T) {
	for header, want := range map[string]string{
		"~ monthly  Rent":     "Rent",
		"~ monthly":           "monthly",
		"~  every 2 weeks   ": "every 2 weeks",
	} {
		if got := ruleName(header); got != want {
			t.Errorf("ruleName(%q) = %q, want %q", header, got, want)
		}
	}
}

const blocksJournal = `; above the rule
~ monthly from 2026-10-01  Vermieter Mueller
    expenses:rent  900,00 EUR
    assets:checking

~ monthly from 2026-10-05  Streaming  ; shared account
    ; a note
    expenses:streaming  13,00 EUR
    assets:checking
2026-01-01 Not a rule
    a  1,00 EUR
    b
~ 2026-01-05  Once-off
    a  1,00 EUR
    b
`

func TestPeriodicRulesReadsEachBlockWithItsPositionAndRawLines(t *testing.T) {
	file := journalWith(t, blocksJournal)

	got, err := file.PeriodicRules()
	if err != nil || len(got) != 3 {
		t.Fatalf("got %d rules, %v; want 3: %+v", len(got), err, got)
	}
	want := domain.PeriodicRule{
		Period: "monthly from 2026-10-01", Description: "Vermieter Mueller", Pos: domain.SourcePos{File: file.path, Line: 2},
		Lines: []string{"~ monthly from 2026-10-01  Vermieter Mueller", "    expenses:rent  900,00 EUR", "    assets:checking"},
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("first rule = %+v, want %+v", got[0], want)
	}
}

func TestPeriodicRulesKeepCommentsInTheirBlockButOutOfTheDescription(t *testing.T) {
	got, _ := journalWith(t, blocksJournal).PeriodicRules()

	second := got[1]
	if second.Description != "Streaming" || second.Pos.Line != 6 || len(second.Lines) != 4 || second.Lines[1] != "    ; a note" {
		t.Errorf("second rule = %+v", second)
	}
	if got[2].Period != "2026-01-05" || got[2].Description != "Once-off" || got[2].Pos.Line != 13 {
		t.Errorf("third rule = %+v", got[2])
	}
}

func TestPeriodicRulesFollowIncludesAndReportUnreadableJournals(t *testing.T) {
	dir := t.TempDir()
	main := writeJournal(t, dir, "main.journal", "include rules.journal\n")
	writeJournal(t, dir, "rules.journal", "~ monthly  Included\n    a  1,00 EUR\n    b\n")

	got, err := main.PeriodicRules()
	if err != nil || len(got) != 1 || filepath.Base(got[0].Pos.File) != "rules.journal" {
		t.Errorf("rules = %+v, %v; want the included one", got, err)
	}
	if _, err := NewFile("/nonexistent/main.journal", septemberOf(2026)).PeriodicRules(); err == nil {
		t.Error("expected an error")
	}
}

func TestRuleNameDropsATrailingComment(t *testing.T) {
	if got := ruleName("~ monthly  Streaming  ; shared account"); got != "Streaming" {
		t.Errorf("ruleName = %q, want Streaming", got)
	}
	if got := ruleName("~ monthly ; just a note"); got != "monthly" {
		t.Errorf("ruleName = %q, want monthly", got)
	}
}
