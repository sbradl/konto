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

// fieldSpec is one field of the form: the name it is shown by and the text of the input it holds, nil for the
// dropdowns, which hold a choice.
type fieldSpec struct {
	label string
	text  func(*periodic.Input) *string
}

// fields are the form's fields, in focus order.
func fields() [fieldCount]fieldSpec {
	return [fieldCount]fieldSpec{
		fieldDate:        {"next due date", func(in *periodic.Input) *string { return &in.Date }},
		fieldFrequency:   {"frequency", nil},
		fieldRemaining:   {"remaining payments", func(in *periodic.Input) *string { return &in.Remaining }},
		fieldDescription: {"description", func(in *periodic.Input) *string { return &in.Description }},
		fieldAmount:      {"amount", func(in *periodic.Input) *string { return &in.Amount }},
		fieldCurrency:    {"currency", nil},
		fieldFrom:        {"from account", func(in *periodic.Input) *string { return &in.From }},
		fieldTo:          {"to account", func(in *periodic.Input) *string { return &in.To }},
	}
}

// keySave saves the form from any field.
const keySave = "ctrl+s"

// Save asks the root to plan the form's rule and write it; Editing is the rule being replaced, nil for a new one.
type Save struct {
	Input   periodic.Input
	Editing *domain.PeriodicRule
}

func (*Save) isAction() {}

// form is the open new-rule or edit-rule form.
type form struct {
	editing    *domain.PeriodicRule
	input      periodic.Input
	focus      int
	back       int // how far the caret of the text input with the focus stands before its end
	currencies []string
	accounts   []string          // the open accounts the pickers offer
	held       map[string]string // the first currency each account holds
	filter     string            // what is typed into the focused account picker
	cursor     layout.Cursor     // the match under the cursor in the picker
	browsing   bool              // the picker's cursor was moved: Enter takes the match
	message    string            // why the last save was refused
}

// text is the text or account field's text as the input holds it.
func (f *form) text(field int) string { return *fields()[field].text(&f.input) }

// Capturing reports whether the form has the keyboard: letters and digits are then text, not view keys.
func (m Model) Capturing() bool { return m.form != nil || m.copying != nil }

// OpenNew opens an empty form for a new rule.
func (m Model) OpenNew() Model {
	return m.open(periodic.Input{Frequency: periodic.Monthly}, nil)
}

// OpenEdit opens the form filled with the rule's fields.
func (m Model) OpenEdit(input periodic.Input, rule domain.PeriodicRule) Model {
	return m.open(input, &rule)
}

func (m Model) open(input periodic.Input, editing *domain.PeriodicRule) Model {
	accounts, held := openAccounts(m.ledger.Accounts)

	opened := &form{editing: editing, input: input, currencies: m.ledger.Commodities, accounts: accounts, held: held}
	if opened.input.Currency == "" && len(opened.currencies) > 0 {
		opened.input.Currency = opened.currencies[0]
	}
	m.form = opened

	return m
}

// openAccounts are the names of the accounts still open, by name, and the first currency each of them holds.
func openAccounts(accounts []domain.Account) (names []string, held map[string]string) {
	held = map[string]string{}
	for _, account := range accounts {
		if account.IsClosed() {
			continue
		}

		names = append(names, account.Name)
		if len(account.Balance) > 0 {
			held[account.Name] = account.Balance[0].Commodity
		}
	}
	slices.Sort(names)

	return names, held
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

// FormKey applies a key to the open form: Tab and Shift+Tab, or the up and down arrows, move between the
// fields (on an account field the arrows move through its matches), Enter and Ctrl+S save, Esc cancels;
// anything else edits the field with the focus.
func (m Model) FormKey(key string) (Model, Action) {
	if m.copying != nil {
		return m.copyKey(key), nil
	}

	if key == layout.KeyEsc {
		return m.CloseForm(), nil
	}

	next := *m.form
	next.message = ""

	if key == keySave || key == layout.KeyEnter && !next.picks() {
		return m.formSave(&next)
	}
	next.key(key)
	m.form = &next

	return m, nil
}

// key moves the focus, picks the account under the picker's cursor or edits the field with the focus.
func (f *form) key(key string) {
	switch key {
	case layout.KeyTab, layout.KeyBackTab:
		f.move(key)
	case layout.KeyUp, layout.KeyDown:
		if f.isAccount() {
			f.edit(key)
		} else {
			f.move(key)
		}
	case layout.KeyEnter:
		f.pick()
	default:
		f.edit(key)
	}
}

// formSave asks the root to plan the rule the form holds.
func (m Model) formSave(f *form) (Model, Action) {
	m.form = f

	return m, &Save{Input: f.input, Editing: f.editing}
}

// shown reports whether the field is on the form: a rule that happens once has no payments left to count.
func (f *form) shown(field int) bool {
	return field != fieldRemaining || f.input.Frequency != periodic.Once
}

// move puts the focus on the next field that is shown, or with Shift+Tab or up the previous one.
func (f *form) move(key string) {
	step := 1
	if key == layout.KeyBackTab || key == layout.KeyUp {
		step = -1
	}
	for {
		f.focus = (f.focus + step + fieldCount) % fieldCount
		if f.shown(f.focus) {
			break
		}
	}
	f.filter, f.back, f.cursor, f.browsing = "", 0, layout.Cursor{}, false
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

	return f.filter != "" || f.browsing || f.text(f.focus) == ""
}

// pick gives the focused field the match under the cursor and moves on, or says there is none.
func (f *form) pick() {
	matches := f.matches()
	if len(matches) == 0 {
		f.message = "no account matches " + f.filter

		return
	}
	chosen := matches[f.cursor.Row] // the cursor never leaves the matches

	*fields()[f.focus].text(&f.input) = chosen
	if f.focus == fieldFrom {
		f.input.Currency = f.currencyHeldBy(chosen)
	}
	f.move(layout.KeyTab)
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
	default: // a text field
		*fields()[f.focus].text(&f.input), f.back, _ = layout.Edit(f.text(f.focus), f.back, key)
	}
}

// editPicker moves the picker's cursor by a cursor key, or types into its filter.
func (f *form) editPicker(key string) {
	if layout.Moves(key) {
		f.cursor, f.browsing = f.cursor.Move(key, len(f.matches()), pickerRows), true

		return
	}

	text, back, _ := layout.Edit(f.filter, f.back, key)
	if text != f.filter {
		f.cursor, f.browsing = layout.Cursor{}, false
	}
	f.filter, f.back = text, back
}

// cycle is the choice a left or right arrow moves to, staying at the ends.
func cycle[T comparable](choices []T, current T, key string) T {
	index := slices.Index(choices, current)
	switch key {
	case layout.KeyLeft:
		index--
	case layout.KeyRight:
		index++
	default:
		return current
	}

	if len(choices) == 0 {
		return current
	}

	return choices[max(0, min(index, len(choices)-1))]
}
