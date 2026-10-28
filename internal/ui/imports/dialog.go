package imports

import (
	"regexp"
	"slices"
	"strings"

	"github.com/sbradl/konto/internal/csvimport"
	"github.com/sbradl/konto/internal/ui/layout"
)

// dialogKind is which categorising dialog, if any, is open over the row table.
type dialogKind int

const (
	dialogNone dialogKind = iota
	dialogChoice
	dialogRuleForm
	dialogRuleList
	dialogAddForm
	dialogPicker
	dialogAccount // naming the account a new IBAN stands for
)

// Key names the view reacts to.
const (
	keyEsc   = "esc"
	keyEnter = "enter"
	keyUp    = "up"
	keyDown  = "down"
	keyHome  = "home"
	keyEnd   = "end"
)

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

// dialog is the state of the dialog that is open. The picker can open over the rule form, which waits
// in Model.parent for the category.
type dialog struct {
	kind     dialogKind
	cursor   int
	filter   string              // the picker's and the rule list's text input
	draft    csvimport.Draft     // the new-rule form
	category csvimport.Candidate // the category the new-rule form has been given
	focus    int                 // the form field with the cursor
	matcher  string              // the add-to-rule form's pattern
	ruleID   string              // the rule being widened
	refusal  string              // why the last confirm was refused
}

// Capturing reports whether a dialog has the keyboard: letters then are text, not view keys.
func (m Model) Capturing() bool { return m.dlg.kind != dialogNone }

// choices are the categorise popup's options for the row under the cursor.
func (m Model) choices() []string {
	if m.staged.Rows[m.table.Row].NeedsCategory() {
		return []string{choiceNewRule, choiceAddToRule, choiceOnce}
	}
	return []string{choiceOnce}
}

// dialogKey applies a key to the open dialog; Esc closes it.
func (m Model) dialogKey(key string) Model {
	if key == keyEsc {
		return m.close()
	}
	handlers := map[dialogKind]func(Model, string) Model{
		dialogChoice: Model.choiceKey, dialogRuleForm: Model.formKey, dialogRuleList: Model.ruleListKey,
		dialogAddForm: Model.addFormKey, dialogPicker: Model.pickerDialogKey, dialogAccount: Model.accountKey,
	}
	return handlers[m.dlg.kind](m, key)
}

// close shuts the open dialog: a picker over the rule form goes back to the form, anything else is
// abandoned along with what was typed.
func (m Model) close() Model {
	if m.dlg.kind == dialogAccount {
		return m.Cleared()
	}
	if m.dlg.kind == dialogPicker && m.parent.kind != dialogNone {
		m.dlg, m.parent = m.parent, dialog{}
		return m
	}
	m.dlg, m.parent = dialog{}, dialog{}
	return m
}

// moveCursor moves a list cursor by an arrow key within count entries.
func moveCursor(cursor int, key string, count int) int {
	switch key {
	case keyUp:
		cursor--
	case keyDown:
		cursor++
	case keyHome:
		cursor = 0
	case keyEnd:
		cursor = count // clamped below
	}
	return max(0, min(cursor, count-1))
}

func (m Model) choiceKey(key string) Model {
	options := m.choices()
	if key != keyEnter {
		m.dlg.cursor = moveCursor(m.dlg.cursor, key, len(options))
		return m
	}
	switch options[m.dlg.cursor] {
	case choiceNewRule:
		m.dlg = dialog{kind: dialogRuleForm, draft: m.staged.DraftFor(m.table.Row)}
	case choiceAddToRule:
		m.dlg = dialog{kind: dialogRuleList}
	default:
		m.dlg = dialog{kind: dialogPicker}
	}
	return m
}

// currentRule is the rule the new-rule form would stage, with the category it has so far.
func (m Model) currentRule() csvimport.Rule {
	return m.dlg.draft.Rule(m.staged.NextRuleID(), m.dlg.category.Target)
}

func (m Model) formKey(key string) Model {
	switch key {
	case "tab", keyDown:
		return m.focusField(m.dlg.focus + 1)
	case "shift+tab", keyUp:
		return m.focusField(m.dlg.focus - 1)
	case keyEnter:
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
	m.parent, m.dlg = m.dlg, dialog{kind: dialogPicker}
	return m
}

// confirmRule stages the rule, or shows why it cannot be.
func (m Model) confirmRule() Model {
	staged, err := m.staged.AddRule(m.currentRule())
	if err != nil {
		m.dlg.refusal = err.Error()
		return m
	}
	m.staged, m.dlg = &staged, dialog{}
	return m
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
	if key == keyEnter {
		if len(rules) == 0 {
			return m
		}
		row := m.staged.Rows[m.table.Row]
		m.dlg = dialog{
			kind: dialogAddForm, ruleID: rules[m.dlg.cursor].ID, matcher: regexp.QuoteMeta(row.Description),
		}
		return m
	}
	if text, ok := layout.Edit(m.dlg.filter, key); ok {
		m.dlg.filter, m.dlg.cursor = text, 0
		return m
	}
	m.dlg.cursor = moveCursor(m.dlg.cursor, key, len(rules))
	return m
}

// addition is the alternative the add-to-rule form would append: the row's payee (else type) matching
// the pattern typed.
func (m Model) addition() []csvimport.Condition {
	return []csvimport.Condition{{Field: m.staged.Rows[m.table.Row].MatchField(), Pattern: m.dlg.matcher}}
}

func (m Model) addFormKey(key string) Model {
	if key == keyEnter {
		staged, err := m.staged.Widen(m.dlg.ruleID, m.addition())
		if err != nil {
			m.dlg.refusal = err.Error()
			return m
		}
		m.staged, m.dlg = &staged, dialog{}
		return m
	}
	m.dlg.refusal = ""
	m.dlg.matcher, _ = layout.Edit(m.dlg.matcher, key)
	return m
}

// accountKey edits the new account's name; Enter gives the account the name, or shows why it cannot have it.
func (m Model) accountKey(key string) Model {
	if key != keyEnter {
		m.dlg.filter, _ = layout.Edit(m.dlg.filter, key)
		m.dlg.refusal = ""
		return m
	}
	named, err := m.staged.WithName(m.dlg.filter)
	if err != nil {
		m.dlg.refusal = err.Error()
		return m
	}
	m.staged, m.dlg = &named, dialog{}
	return m
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
	if key == keyEnter {
		return m.pick(candidates)
	}
	if text, ok := layout.Edit(m.dlg.filter, key); ok {
		m.dlg.filter, m.dlg.cursor, m.dlg.refusal = text, 0, ""
		return m
	}
	m.dlg.cursor = moveCursor(m.dlg.cursor, key, count)
	return m
}

// pick takes the entry under the cursor: a category, or a new one created from what was typed.
func (m Model) pick(candidates []csvimport.Candidate) Model {
	if m.dlg.cursor < len(candidates) {
		return m.picked(*m.staged, candidates[m.dlg.cursor])
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
	if m.parent.kind != dialogNone {
		m.dlg, m.parent = m.parent, dialog{}
		m.dlg.category, m.dlg.refusal = candidate, ""
		m.staged = &staged
		return m
	}
	once := staged.Once(m.table.Row, candidate)
	m.staged, m.dlg = &once, dialog{}
	return m
}
