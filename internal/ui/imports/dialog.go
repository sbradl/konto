package imports

import (
	"regexp"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/ui/layout"
)

// mode is what has the Import view's keys: the staged import's rows (or the empty start), the file picker, or
// one of the categorising dialogs over the rows.
type mode int

const (
	modeRows mode = iota
	modeFiles
	modeChoice   // the categorise popup
	modeRuleForm // the new-rule form
	modeRuleList // the rules to add the row to
	modeAddForm  // the add-to-rule form
	modeCategory // the category picker, for the rule form or for the row alone
	modeAccount  // naming the account a new IBAN stands for
)

// dialog reports whether the mode is a dialog over the rows: it then has the keyboard.
func (m mode) dialog() bool { return m >= modeChoice }

// The choices of the categorise popup.
const (
	choiceNewRule   = "new rule"
	choiceAddToRule = "add to rule"
	choiceOnce      = "once"
)

// The fields of the new-rule form, in focus order.
const (
	fieldPattern = iota
	fieldPurpose
	fieldCategory
	fieldCount
)

// dialog is what the open dialog holds; each mode uses its own fields. The category picker can open over the
// rule form, which waits in Model.waiting for the category.
type dialog struct {
	cursor   layout.Cursor       // the entry selected in the choices, the rule list or the category picker
	filter   string              // the category picker's and the rule list's text input
	draft    csvimport.Draft     // the new-rule form
	category csvimport.Candidate // the category the new-rule form has been given
	focus    int                 // the form field with the cursor
	matcher  string              // the add-to-rule form's pattern
	ruleID   string              // the rule being widened
	name     string              // the name typed for the new account
	refusal  string              // why the last confirm was refused
}

// Capturing reports whether a dialog has the keyboard: letters then are text, not view keys.
func (m Model) Capturing() bool { return m.mode.dialog() }

// show switches to the mode with the dialog state given; what another dialog held, and a waiting form, are
// dropped. Every change of mode goes through it.
func (m Model) show(mode mode, dlg dialog) Model {
	m.mode, m.dlg, m.waiting = mode, dlg, nil
	return m
}

// closeDialog goes back to the rows from a dialog, abandoning what was typed; the file picker stays open.
func (m Model) closeDialog() Model {
	if m.mode.dialog() {
		return m.show(modeRows, dialog{})
	}
	return m
}

// dialogPage is how many lines of the open dialog its popup shows at once.
func (m Model) dialogPage() int {
	_, body := m.dialogBody()
	return layout.PopupCapacity(len(body), m.height)
}

// choices are the categorise popup's options for the row under the cursor.
func (m Model) choices() []string {
	if m.staged.Rows[m.table.Row].NeedsCategory() {
		return []string{choiceNewRule, choiceAddToRule, choiceOnce}
	}
	return []string{choiceOnce}
}

// dialogKey applies a key to the open dialog; Esc closes it.
func (m Model) dialogKey(key string) Model {
	if key == layout.KeyEsc {
		return m.close()
	}
	switch m.mode { // exhaustive: a new dialog without its handler fails the lint
	case modeChoice:
		return m.choiceKey(key)
	case modeRuleForm:
		return m.formKey(key)
	case modeRuleList:
		return m.ruleListKey(key)
	case modeAddForm:
		return m.addFormKey(key)
	case modeCategory:
		return m.pickerDialogKey(key)
	case modeAccount:
		return m.accountKey(key)
	case modeRows, modeFiles:
	}
	return m
}

// close shuts the open dialog: a picker over the rule form goes back to the form, anything else is
// abandoned along with what was typed.
func (m Model) close() Model {
	if m.mode == modeAccount {
		return m.Cleared()
	}
	if m.mode == modeCategory && m.waiting != nil {
		return m.show(modeRuleForm, *m.waiting)
	}
	return m.show(modeRows, dialog{})
}

// moved is the dialog with its list cursor moved by a cursor key within count entries.
func (m Model) moved(key string, count int) Model {
	m.dlg.cursor = m.dlg.cursor.Move(key, count, m.dialogPage())
	return m
}

func (m Model) choiceKey(key string) Model {
	options := m.choices()
	if key != layout.KeyEnter {
		return m.moved(key, len(options))
	}
	switch options[m.dlg.cursor.Row] {
	case choiceNewRule:
		return m.show(modeRuleForm, dialog{draft: m.staged.DraftFor(m.table.Row)})
	case choiceAddToRule:
		return m.show(modeRuleList, dialog{})
	default:
		return m.show(modeCategory, dialog{})
	}
}

// currentRule is the rule the new-rule form would stage, with the category it has so far.
func (m Model) currentRule() csvimport.Rule {
	return m.dlg.draft.Rule(m.staged.NextRuleID(), m.dlg.category.Target)
}

func (m Model) formKey(key string) Model {
	switch key {
	case layout.KeyTab, layout.KeyDown:
		return m.focusField(m.dlg.focus + 1)
	case layout.KeyBackTab, layout.KeyUp:
		return m.focusField(m.dlg.focus - 1)
	case layout.KeyEnter:
		return m.confirmRule()
	}
	m.dlg.refusal = ""
	switch m.dlg.focus {
	case fieldPattern:
		m.dlg.draft.Pattern, _ = layout.Edit(m.dlg.draft.Pattern, key)
	case fieldPurpose:
		m.dlg.draft.Purpose, _ = layout.Edit(m.dlg.draft.Purpose, key)
	}
	return m
}

// focusField moves the form's focus; arriving on the category opens the picker.
func (m Model) focusField(field int) Model {
	m.dlg.focus = (field + fieldCount) % fieldCount
	if m.dlg.focus != fieldCategory {
		return m
	}
	form := m.dlg
	m = m.show(modeCategory, dialog{})
	m.waiting = &form
	return m
}

// confirmRule stages the rule, or shows why it cannot be.
func (m Model) confirmRule() Model {
	staged, err := m.staged.AddRule(m.currentRule())
	if err != nil {
		m.dlg.refusal = err.Error()
		return m
	}
	m.staged = &staged
	return m.show(modeRows, dialog{})
}

// ruleChoices are the rules the list offers: those whose category contains the filter.
func (m Model) ruleChoices() []csvimport.Rule {
	needle := strings.ToLower(m.dlg.filter)
	return slices.DeleteFunc(slices.Clone(m.staged.Rules), func(rule csvimport.Rule) bool {
		return !strings.Contains(strings.ToLower(rule.Category), needle)
	})
}

func (m Model) ruleListKey(key string) Model {
	rules := m.ruleChoices()
	if key == layout.KeyEnter {
		if len(rules) == 0 {
			return m
		}
		row := m.staged.Rows[m.table.Row]
		widen := dialog{ruleID: rules[m.dlg.cursor.Row].ID, matcher: regexp.QuoteMeta(row.Description)}
		return m.show(modeAddForm, widen)
	}
	if text, ok := layout.Edit(m.dlg.filter, key); ok {
		m.dlg.filter, m.dlg.cursor = text, layout.Cursor{}
		return m
	}
	return m.moved(key, len(rules))
}

// addition is the alternative the add-to-rule form would append: the row's payee (else type) matching
// the pattern typed.
func (m Model) addition() []csvimport.Condition {
	return []csvimport.Condition{{Field: m.staged.Rows[m.table.Row].MatchField(), Pattern: m.dlg.matcher}}
}

func (m Model) addFormKey(key string) Model {
	if key == layout.KeyEnter {
		staged, err := m.staged.Widen(m.dlg.ruleID, m.addition())
		if err != nil {
			m.dlg.refusal = err.Error()
			return m
		}
		m.staged = &staged
		return m.show(modeRows, dialog{})
	}
	m.dlg.refusal = ""
	m.dlg.matcher, _ = layout.Edit(m.dlg.matcher, key)
	return m
}

// accountKey edits the new account's name; Enter gives the account the name, or shows why it cannot have it.
func (m Model) accountKey(key string) Model {
	if key != layout.KeyEnter {
		m.dlg.name, _ = layout.Edit(m.dlg.name, key)
		m.dlg.refusal = ""
		return m
	}
	named, err := m.staged.WithName(m.dlg.name)
	if err != nil {
		m.dlg.refusal = err.Error()
		return m
	}
	m.staged = &named
	return m.show(modeRows, dialog{})
}

// pickerEntries are what the picker lists for the filter: the matching candidates, then "+ create" when
// nothing matches or nothing has been typed yet.
func (m Model) pickerEntries() (candidates []csvimport.Candidate, create bool) {
	row := m.staged.Rows[m.table.Row]
	candidates = csvimport.Matching(m.staged.Candidates(row), m.dlg.filter)
	return candidates, len(candidates) == 0 || m.dlg.filter == ""
}

func (m Model) pickerDialogKey(key string) Model {
	candidates, create := m.pickerEntries()
	count := len(candidates)
	if create {
		count++
	}
	if key == layout.KeyEnter {
		return m.pick(candidates)
	}
	if text, ok := layout.Edit(m.dlg.filter, key); ok {
		m.dlg.filter, m.dlg.cursor, m.dlg.refusal = text, layout.Cursor{}, ""
		return m
	}
	return m.moved(key, count)
}

// pick takes the entry under the cursor: a category, or a new one created from what was typed.
func (m Model) pick(candidates []csvimport.Candidate) Model {
	if m.dlg.cursor.Row < len(candidates) {
		return m.picked(*m.staged, candidates[m.dlg.cursor.Row])
	}
	row := m.staged.Rows[m.table.Row]
	staged, candidate, err := m.staged.Create(csvimport.SuggestName(row, m.dlg.filter))
	if err != nil {
		m.dlg.refusal = err.Error()
		return m
	}
	return m.picked(staged, candidate)
}

// picked closes the picker with the category: for the waiting form, or for this row alone.
func (m Model) picked(staged csvimport.Staged, candidate csvimport.Candidate) Model {
	if m.waiting != nil {
		m = m.show(modeRuleForm, *m.waiting)
		m.dlg.category, m.dlg.refusal = candidate, ""
		m.staged = &staged
		return m
	}
	once := staged.Once(m.table.Row, candidate)
	m.staged = &once
	return m.show(modeRows, dialog{})
}
