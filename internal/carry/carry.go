// Package carry is the use case that keeps balances right across year files (ADR 0013): a write that
// crosses into a new year, or changes a year a later file already carries into, also rewrites the managed
// closing and opening transactions between the files, with the balances hledger adds up.
package carry

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sbradl/konto/internal/domain"
)

type journalFile interface {
	CarryYears(changeset domain.Changeset) ([]int, error)
	Render(changeset domain.Changeset) (string, error)
	Apply(changeset domain.Changeset) error
}

type balances interface {
	TextBalances(ctx context.Context, text string, before time.Time) ([]domain.Account, error)
}

// Writer is a journal file whose writes keep the carry right.
type Writer struct {
	file journalFile
	gw   balances
}

// NewWriter returns a Writer that writes through file and asks gw for the balances to carry.
func NewWriter(file journalFile, gw balances) *Writer { return &Writer{file: file, gw: gw} }

// Apply writes the changeset together with the carry it makes stale.
func (w *Writer) Apply(changeset domain.Changeset) error {
	complete, err := w.complete(context.Background(), changeset)
	if err != nil {
		return err
	}
	return w.file.Apply(complete) //nolint:wrapcheck // the journal's own failure says what went wrong
}

// Render is the journal as it would read after Apply.
func (w *Writer) Render(changeset domain.Changeset) (string, error) {
	complete, err := w.complete(context.Background(), changeset)
	if err != nil {
		return "", err
	}
	return w.file.Render(complete) //nolint:wrapcheck // the journal's own failure says what went wrong
}

// complete adds a SetCarry for each stale year, oldest first: the balances at the start of the year, counted
// on the journal as the changeset leaves it without that year's carry, so the closing carry never cancels them.
func (w *Writer) complete(ctx context.Context, changeset domain.Changeset) (domain.Changeset, error) {
	years, err := w.file.CarryYears(changeset)
	if err != nil {
		return domain.Changeset{}, fmt.Errorf("finding the carry to write: %w", err)
	}
	if len(years) == 0 {
		return changeset, nil
	}
	edits := slices.Clone(changeset.Edits)
	carried := false
	for _, year := range years {
		text, err := w.file.Render(domain.Changeset{Edits: append(slices.Clone(edits), domain.SetCarry{Year: year})})
		if err != nil {
			return domain.Changeset{}, fmt.Errorf("preparing the carry into %d: %w", year, err)
		}
		balances, err := w.gw.TextBalances(ctx, text, time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC))
		if err != nil {
			return domain.Changeset{}, fmt.Errorf("adding up the carry into %d: %w", year, err)
		}
		edits = append(edits, domain.SetCarry{Year: year, Balances: balances})
		carried = carried || len(balances) > 0
	}
	if carried {
		edits = append(edits,
			domain.Declare{Directive: "account", Name: domain.CarryAccount},
			domain.Declare{Directive: "payee", Name: domain.OpeningDescription},
			domain.Declare{Directive: "payee", Name: domain.ClosingDescription},
			domain.Declare{Directive: "tag", Name: "konto"},
		)
	}
	return domain.Changeset{Edits: edits}, nil
}
