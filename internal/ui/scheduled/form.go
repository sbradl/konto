package scheduled

import (
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/domain"
	"github.com/sbradl/konto/internal/periodic"
	"github.com/sbradl/konto/internal/ui/layout"
)

// The fields of the form, in focus order.
const (
	fieldDate = iota
	fieldFrequency
	fieldRemaining
	fieldDescription
	fieldAmount
	fieldCurrency
	fieldFrom
	fieldTo
	fieldCount
)

// fieldLabel is the name a field is shown by.
func fieldLabel(field int) string {
	return [fieldCount]string{
		"next due date", "frequency", "remaining payments", "description", "amount", "currency",
		"from account", "to account",
	}[field]
}

// Key names the form reacts to.
const (
	keyEsc   = "esc"
	keyEnter = "enter"
	keyTab   = "tab"
	keyBack  = "shift+tab"
	keyUp    = "up"
	keyDown  = "down"
	keyLeft  = "left"
	keyRight = "right"
	keySave  = "ctrl+s"
)

// Save asks the root to plan the form's rule and write it; Editing is the rule being replaced, nil for a new one.
type Save struct {
	Input   periodic.Input
	Editing *domain.PeriodicRule
}

// Action is what the form asks the root to do; the zero value asks for nothing.
type Action struct{ Save *Save }

// form is the open new-rule or edit-rule form.
type form struct {
	editing    *domain.PeriodicRule
	input      periodic.Input
	focus      int
	currencies []string
	accounts   []string          // the open accounts the pickers offer
	held       map[string]string // the first currency each account holds
	filter     string            // what is typed into the focused account picker
	cursor     int               // the match under the cursor in the picker
	browsing   bool              // the picker's cursor was moved: Enter takes the match
	message    string            // why the last save was refused
}

// Capturing reports whether the form has the keyboard: letters and digits are then text, not view keys.
func (m Model) Capturing() bool { return m.form != nil }

// OpenNew opens an empty form for a new rule.
func (m Model) OpenNew() Model {
	return m.open(periodic.Input{Frequency: periodic.Monthly}, nil)
}

// OpenEdit opens the form filled with the rule's fields.
func (m Model) OpenEdit(input periodic.Input, rule domain.PeriodicRule) Model {
	return m.open(input, &rule)
}

func (m Model) open(input periodic.Input, editing *domain.PeriodicRule) Model {
	opened := &form{editing: editing, input: input, currencies: m.ledger.Commodities, held: map[string]string{}}
	for _, account := range m.ledger.Accounts {
		if account.IsClosed() {
			continue
		}
		opened.accounts = append(opened.accounts, account.Name)
		if len(account.Balance) > 0 {
			opened.held[account.Name] = account.Balance[0].Commodity
		}
	}
	slices.Sort(opened.accounts)
	if opened.input.Currency == "" && len(opened.currencies) > 0 {
		opened.input.Currency = opened.currencies[0]
	}
	m.form = opened
	return m
}

// CloseForm drops the form, whatever it holds.
func (m Model) CloseForm() Model {
	m.form = nil
	return m
}

// WithFormError keeps the form open and says why its rule was not saved.
func (m Model) WithFormError(text string) Model {
	if m.form != nil {
		next := *m.form
		next.message = text
		m.form = &next
	}
	return m
}

// FormKey applies a key to the open form: Tab and Shift+Tab move between the fields, Enter and Ctrl+S save,
// Esc cancels; anything else edits the field with the focus.
func (m Model) FormKey(key string) (Model, Action) {
	next := *m.form
	next.message = ""
	switch key {
	case keyEsc:
		return m.CloseForm(), Action{}
	case keySave:
		return m.formSave(&next)
	case keyTab, keyBack:
		next.move(key)
	case keyEnter:
		if !next.picks() {
			return m.formSave(&next)
		}
		next.pick()
	default:
		next.edit(key)
	}
	m.form = &next
	return m, Action{}
}

// formSave asks the root to plan the rule the form holds.
func (m Model) formSave(f *form) (Model, Action) {
	m.form = f
	return m, Action{Save: &Save{Input: f.input, Editing: f.editing}}
}

// shown reports whether the field is on the form: a rule that happens once has no payments left to count.
func (f *form) shown(field int) bool {
	return field != fieldRemaining || f.input.Frequency != periodic.Once
}

// move puts the focus on the next or the previous field that is shown.
func (f *form) move(key string) {
	step := 1
	if key == keyBack {
		step = -1
	}
	for {
		f.focus = (f.focus + step + fieldCount) % fieldCount
		if f.shown(f.focus) {
			break
		}
	}
	f.filter, f.cursor, f.browsing = "", 0, false
}

// isAccount reports that the focus is on an account picker.
func (f *form) isAccount() bool { return f.focus == fieldFrom || f.focus == fieldTo }

// matches are the accounts whose name holds what is typed, case aside.
func (f *form) matches() []string {
	needle := strings.ToLower(f.filter)
	return slices.DeleteFunc(slices.Clone(f.accounts), func(name string) bool {
		return !strings.Contains(strings.ToLower(name), needle)
	})
}

// picks reports that Enter takes a match of the picker instead of saving: something was typed, the cursor was
// moved, or the field has no account yet.
func (f *form) picks() bool {
	if !f.isAccount() {
		return false
	}
	return f.filter != "" || f.browsing || f.value() == ""
}

// value is the field with the focus, as the input holds it.
func (f *form) value() string {
	return [fieldCount]string{
		f.input.Date, "", f.input.Remaining, f.input.Description, f.input.Amount, f.input.Currency,
		f.input.From, f.input.To,
	}[f.focus]
}

// pick gives the focused field the match under the cursor and moves on, or says there is none.
func (f *form) pick() {
	matches := f.matches()
	if len(matches) == 0 {
		f.message = "no account matches " + f.filter
		return
	}
	chosen := matches[f.cursor] // the cursor never leaves the matches
	if f.focus == fieldFrom {
		f.input.From = chosen
		f.input.Currency = f.currencyHeldBy(chosen)
	} else {
		f.input.To = chosen
	}
	f.move(keyTab)
}

// currencyHeldBy is the first currency the account holds when the journal declares it, else the first declared.
func (f *form) currencyHeldBy(account string) string {
	if held := f.held[account]; slices.Contains(f.currencies, held) {
		return held
	}
	if len(f.currencies) > 0 {
		return f.currencies[0]
	}
	return f.input.Currency
}

// edit applies a typed key to the field with the focus.
func (f *form) edit(key string) {
	switch f.focus {
	case fieldFrequency:
		f.input.Frequency = cycle(periodic.Frequencies(), f.input.Frequency, key)
	case fieldCurrency:
		f.input.Currency = cycle(f.currencies, f.input.Currency, key)
	case fieldFrom, fieldTo:
		f.editPicker(key)
	default:
		text, _ := layout.Edit(f.value(), key)
		f.setText(text)
	}
}

// setText stores the text of the text field with the focus.
func (f *form) setText(text string) {
	switch f.focus {
	case fieldDate:
		f.input.Date = text
	case fieldRemaining:
		f.input.Remaining = text
	case fieldDescription:
		f.input.Description = text
	case fieldAmount:
		f.input.Amount = text
	}
}

// editPicker types into the picker's filter or moves its cursor.
func (f *form) editPicker(key string) {
	switch key {
	case keyUp:
		f.cursor, f.browsing = max(0, f.cursor-1), true
	case keyDown:
		f.cursor, f.browsing = min(len(f.matches())-1, f.cursor+1), true
	default:
		f.filter, _ = layout.Edit(f.filter, key)
		f.cursor, f.browsing = 0, false
	}
}

// cycle is the choice a left or right arrow (or up or down) moves to, staying at the ends.
func cycle[T comparable](choices []T, current T, key string) T {
	index := slices.Index(choices, current)
	switch key {
	case keyLeft, keyUp:
		index--
	case keyRight, keyDown:
		index++
	default:
		return current
	}
	if len(choices) == 0 {
		return current
	}
	return choices[max(0, min(index, len(choices)-1))]
}
