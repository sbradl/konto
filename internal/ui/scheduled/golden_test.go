package scheduled

import (
	"fmt"
	"testing"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

// golden compares the plain view with testdata/<name>.txt; KONTO_UPDATE_GOLDEN=1 rewrites the file.
func golden(t *testing.T, name string, model Model) {
	t.Helper()
	uitest.Golden(t, name, screen(model))
}

func manyRules(count int) domain.Ledger {
	ledger := household()
	for i := range count {
		ledger.Rules = append(ledger.Rules, rule(100+i, "monthly from 2026-10-05", fmt.Sprintf("Rule %02d", i),
			"    expenses:x  1,00 EUR", "    assets:checking"))
	}

	return ledger
}

func TestTheListViewIsLaidOutAsTheGoldenFilesShow(t *testing.T) {
	golden(t, "household-180x30", loaded())
	golden(t, "household-100x14", New().Resize(100, 14).WithLedger(household()))
	golden(t, "household-100x11", New().Resize(100, 11).WithLedger(household()))
	golden(t, "household-100x9", New().Resize(100, 9).WithLedger(household()))
	golden(t, "household-100x17", New().Resize(100, 17).WithLedger(household()))
	golden(t, "five-rules-100x30", New().Resize(100, 30).WithLedger(manyRules(2)))
	golden(t, "empty-100x20", New().Resize(100, 20).WithLedger(domain.Ledger{}))
	golden(t, "many-rules-120x24", New().Resize(120, 24).WithLedger(manyRules(12)))
	scrolled := New().Resize(120, 24).WithLedger(manyRules(12))
	for range 8 {
		scrolled = scrolled.Move("down")
	}
	golden(t, "many-rules-scrolled", scrolled)
	golden(t, "many-rules-end", scrolled.Move("end"))
	golden(t, "many-rules-paged", scrolled.Move("pgup"))
}

func TestTheFormViewIsLaidOutAsTheGoldenFilesShow(t *testing.T) {
	ledger := withAccounts()
	for _, name := range []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"} {
		ledger.Accounts = append(ledger.Accounts, domain.Account{Name: "zz:" + name})
	}
	picking := New().Resize(120, 40).WithLedger(ledger).OpenNew()
	picking, _ = pressed(picking, "tab", "tab", "tab", "tab", "tab", "tab")
	picking = typed(picking, "zz:")

	golden(t, "form-new-120x40", New().Resize(120, 40).WithLedger(withAccounts()).OpenNew())
	golden(t, "form-new-narrow", New().Resize(60, 40).WithLedger(withAccounts()).OpenNew())
	golden(t, "form-picker-first", picking)
	pressedDown, _ := pressed(picking, "down", "down", "down")
	golden(t, "form-picker-fourth", pressedDown)
	pressedDown, _ = pressed(pressedDown, "down", "down")
	golden(t, "form-picker-fifth", pressedDown)
	pressedDown, _ = pressed(pressedDown, "down", "down", "down", "down")
	golden(t, "form-picker-last", pressedDown)
	nothing := typed(picking, "x:")
	golden(t, "form-picker-no-match", nothing)
	cleared, _ := pressed(picking, "ctrl+u")
	golden(t, "form-picker-create", typed(cleared, "gym"))
}
