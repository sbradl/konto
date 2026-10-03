package journal

import (
	"reflect"
	"testing"
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
	if _, err := NewFile("/nonexistent/main.journal").RulesReferencing("assets:spare"); err == nil {
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
