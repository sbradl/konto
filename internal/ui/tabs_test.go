package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/ui/uitest"
)

// Tab indices the tests name.
const (
	importTab    = 1
	scheduledTab = 2
)

func TestATabThatCapturesKeysHandsThemOn(t *testing.T) {
	for _, tab := range tabs() {
		if tab.capture != nil && (tab.capture.active == nil || tab.capture.key == nil) {
			t.Errorf("%s: a capture needs both when it is active and what takes its keys", tab.name)
		}

		if len(tab.keys) > 0 && tab.key == nil {
			t.Errorf("%s: hints keys but handles none", tab.name)
		}
	}
}

// scrolledDashboard has its Activity scrolled away from today, for t to bring it back, and the focus on Accounts.
func scrolledDashboard(t *testing.T) Model {
	t.Helper()
	ledger := accountsLedger()
	for range 60 { // more than Activity holds
		ledger.Transactions = append(ledger.Transactions, domain.Transaction{Description: "Shop"})
	}
	model := withLedger(t, ledger, &fakePlanner{changeset: closeSpare}, &fakeWriter{})
	for _, code := range []rune{tea.KeyTab, tea.KeyEnd, tea.KeyTab} {
		model = pressKey(model, code)
	}

	return model
}

// Every key a tab hints at does something on a view that has something to do it to: the hints and the views'
// key switches stay in step.
func TestEveryHintedKeyIsHandled(t *testing.T) {
	rigs := map[string]func(*testing.T) Model{
		"dash": scrolledDashboard,
		"import": func(t *testing.T) Model {
			t.Helper()

			return stagedForWrite(t, writable()).model
		},
		"sched": func(t *testing.T) Model {
			t.Helper()

			return scheduledWithForm(t, &fakeRules{}).model
		},
	}
	for _, tab := range tabs() {
		if len(tab.keys) == 0 {
			continue
		}

		rig, ok := rigs[tab.name]
		if !ok {
			t.Errorf("%s: no rig to check its hinted keys on", tab.name)

			continue
		}
		for _, binding := range tab.keys {
			model := rig(t)

			pressed, cmd := press(model, binding.Key)
			if cmd == nil && uitest.Plain(pressed.View().Content) == uitest.Plain(model.View().Content) {
				t.Errorf("%s: %s is hinted but does nothing", tab.name, binding.Key)
			}
		}
	}
}
