package domain

// DeleteRule removes the `~` block that starts at Pos: its line and the indented lines under it,
// comments within the block included. Comment lines above it stay.
type DeleteRule struct {
	Pos SourcePos
}

// AddRule appends a `~` block to a journal file after a blank line. File is the path hledger reports for the
// file; empty means the main journal.
type AddRule struct {
	File  string
	Lines []string
}

// ReplaceRule replaces the `~` block that starts at Pos: its line and the indented lines under it. Comment
// lines above it stay.
type ReplaceRule struct {
	Pos   SourcePos
	Lines []string
}

func (DeleteRule) isEdit()  {}
func (AddRule) isEdit()     {}
func (ReplaceRule) isEdit() {}
