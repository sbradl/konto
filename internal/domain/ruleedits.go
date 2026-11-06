package domain

// DeleteRule removes the `~` block that starts at Pos: its line and the indented lines under it,
// comments within the block included. Comment lines above it stay.
type DeleteRule struct {
	Pos SourcePos
	Was string // the block as it was read (PeriodicRule.Entry): a journal changed under konto is refused
}

// AddRule appends a `~` block to the main journal after a blank line (ADR 0013: rules go to main.journal).
type AddRule struct {
	Lines []string
}

// ReplaceRule replaces the `~` block that starts at Pos: its line and the indented lines under it. Comment
// lines above it stay.
type ReplaceRule struct {
	Pos   SourcePos
	Was   string // the block as it was read (PeriodicRule.Entry): a journal changed under konto is refused
	Lines []string
}

func (DeleteRule) isEdit()  {}
func (AddRule) isEdit()     {}
func (ReplaceRule) isEdit() {}
