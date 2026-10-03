package domain

// Changeset is the one write model (ADR 0012): a list of targeted edits to the journal text,
// produced by an aggregate or use case and applied by the journal adapter.
type Changeset struct {
	Edits []Edit
}

// Edit is one targeted change. The set is closed: only this package implements it.
type Edit interface{ isEdit() }

// SetAccountTag puts a tag on an account's directive, replacing any tag with the same key.
// The directive is created when the account has none.
type SetAccountTag struct {
	Account string
	Key     string // e.g. "konto:closed"
	Value   string // e.g. "2026-09-30"
}

// RemoveAccountTag takes the tag with the given key off an account's directive.
type RemoveAccountTag struct {
	Account string
	Key     string
}

// DeleteRule removes the `~` block that starts at Pos: its line and the indented lines under it,
// comments within the block included. Comment lines above it stay.
type DeleteRule struct {
	Pos SourcePos
}

func (SetAccountTag) isEdit()    {}
func (RemoveAccountTag) isEdit() {}
func (DeleteRule) isEdit()       {}
