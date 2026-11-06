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

// Writer is a journal file whose writes keep the carry right: Prepare completes a changeset with its carry
// once, and that same completed changeset is checked and then applied, so what is written is what was checked.
type Writer struct {
	file journalFile
	gw   balances
}

// NewWriter returns a Writer that writes through file and asks gw for the balances to carry.
func NewWriter(file journalFile, gw balances) *Writer { return &Writer{file: file, gw: gw} }

// Apply writes a changeset Prepare completed.
func (w *Writer) Apply(changeset domain.Changeset) error {
	return w.file.Apply(changeset) //nolint:wrapcheck // the journal's own failure says what went wrong
}

// Prepare is the changeset with the carry it makes stale: a SetCarry for each stale year, oldest first, of the
// balances at the start of the year, counted on the journal as the changeset leaves it without that year's
// carry, so the closing carry never cancels them.
func (w *Writer) Prepare(ctx context.Context, changeset domain.Changeset) (domain.Changeset, error) {
	years, err := w.file.CarryYears(changeset)
	if err != nil {
		return domain.Changeset{}, fmt.Errorf("finding the carry to write: %w", err)
	}

	if len(years) == 0 {
		return changeset, nil
	}

	edits, carried, err := w.carryInto(ctx, slices.Clone(changeset.Edits), years)
	if err != nil {
		return domain.Changeset{}, err
	}

	if carried {
		edits = append(edits,
			domain.DeclareAccount(domain.OpeningBalanceAccount, ""),
			domain.DeclarePayee(domain.OpeningDescription),
			domain.DeclarePayee(domain.ClosingDescription),
			domain.DeclareTag(domain.KontoTag),
		)
	}

	return domain.Changeset{Edits: edits}, nil
}

// carryInto appends a SetCarry for each year to edits, and reports whether any of them carries a balance.
func (w *Writer) carryInto(ctx context.Context, edits []domain.Edit, years []int) ([]domain.Edit, bool, error) {
	carried := false
	for _, year := range years {
		balances, err := w.balancesInto(ctx, edits, year)
		if err != nil {
			return nil, false, err
		}
		edits = append(edits, domain.SetCarry{Year: year, Balances: balances})
		carried = carried || len(balances) > 0
	}

	return edits, carried, nil
}

// balancesInto are the balances at the start of year, on the journal as edits leave it without that year's carry.
func (w *Writer) balancesInto(ctx context.Context, edits []domain.Edit, year int) ([]domain.Account, error) {
	text, err := w.file.Render(domain.Changeset{Edits: append(slices.Clone(edits), domain.SetCarry{Year: year})})
	if err != nil {
		return nil, fmt.Errorf("preparing the carry into %d: %w", year, err)
	}

	balances, err := w.gw.TextBalances(ctx, text, time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, fmt.Errorf("adding up the carry into %d: %w", year, err)
	}

	return balances, nil
}
