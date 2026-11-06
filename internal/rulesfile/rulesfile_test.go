package rulesfile

import "testing"

// A rule's block runs from the blank line before its id line to the blank line after it.
func TestBlockIsTheParagraphAroundTheRulesIDLine(t *testing.T) {
	lines := []string{
		"if %payee A", "    comment2 konto-rule:1", "",
		"if %payee B", "& %purpose C", "    account2 expenses:b", "    comment2 konto-rule:2", "    x", "",
		"if %payee D", "    comment2 konto-rule:3",
	}
	for _, tt := range []struct {
		id         string
		first, end int
		found      bool
	}{{"1", 0, 2, true}, {"2", 3, 8, true}, {"3", 9, 11, true}, {"4", 0, 0, false}} {
		first, end, found := Block(lines, tt.id)
		if first != tt.first || end != tt.end || found != tt.found {
			t.Errorf("rule %s: Block = %d, %d, %v; want %d, %d, %v", tt.id, first, end, found, tt.first, tt.end, tt.found)
		}
	}
	idFirst := []string{"comment2 konto-rule:5", "    account2 x", "", "comment2 konto-rule:6"}
	for id, want := range map[string][2]int{"5": {0, 2}, "6": {3, 4}} {
		if first, end, found := Block(idFirst, id); !found || [2]int{first, end} != want {
			t.Errorf("rule %s opening its block: Block = %d, %d, %v; want %v", id, first, end, found, want)
		}
	}
}

// A line of nothing but spaces ends a block too, as it does when the rules are read.
func TestBlockEndsAtALineOfSpaces(t *testing.T) {
	lines := []string{"if %payee A", "    comment2 konto-rule:1", "  ", "if %payee B", "    comment2 konto-rule:2", "\t"}

	if first, end, found := Block(lines, "2"); !found || first != 3 || end != 5 {
		t.Errorf("Block = %d, %d, %v; want 3, 5", first, end, found)
	}

	if !IsBlank(" \t") || IsBlank(" x") {
		t.Error("only a line of spaces is blank")
	}
}
