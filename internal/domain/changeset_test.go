package domain

import "testing"

// Every edit kind is an Edit; the closed set is what lets the journal adapter switch over it.
func TestEditKindsAreEdits(*testing.T) {
	for _, edit := range []Edit{SetAccountTag{}, RemoveAccountTag{}} {
		edit.isEdit()
	}
}
