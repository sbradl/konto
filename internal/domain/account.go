package domain

import (
	"strings"
	"time"
)

// Source is how an account's transactions get into the journal. The zero value
// is unclassified: no konto:source tag on the account directive.
type Source int

// The sources an asset or liability account can have (spec §5.3).
const (
	SourceNone Source = iota
	SourceManual
	SourceImport
	SourceClearing
)

// ParseSource reads the value of a konto:source tag; anything unknown is unclassified.
func ParseSource(value string) Source {
	switch value {
	case "manual":
		return SourceManual
	case "import":
		return SourceImport
	case "clearing":
		return SourceClearing
	}
	return SourceNone
}

// Account is one postable account with its current balance.
type Account struct {
	Name    string
	Source  Source
	IBAN    string
	Closed  time.Time // the zero time while the account is open
	Balance MultiAmount
}

// IsClosed reports whether the account carries a konto:closed date.
func (a Account) IsClosed() bool { return !a.Closed.IsZero() }

// IsClassifiable reports whether the account needs a source: only assets and
// liabilities do; categories and equity never do.
func (a Account) IsClassifiable() bool { return HoldsMoney(a.Name) }

// IsUnclassified reports an asset or liability without a source.
func (a Account) IsUnclassified() bool { return a.IsClassifiable() && a.Source == SourceNone }

// Declaration is what an `account` directive says about an account, read from journal text.
type Declaration struct {
	Name   string
	Source Source
	IBAN   string
	Closed time.Time
}

// ApplyTag records one konto tag from the directive's comment; other tags are ignored.
func (d *Declaration) ApplyTag(tag string) {
	switch {
	case strings.HasPrefix(tag, "konto:source:"):
		d.Source = ParseSource(strings.TrimPrefix(tag, "konto:source:"))
	case strings.HasPrefix(tag, "konto:iban:"):
		d.IBAN = strings.TrimPrefix(tag, "konto:iban:")
	case strings.HasPrefix(tag, "konto:closed:"):
		if date, err := time.Parse(time.DateOnly, strings.TrimPrefix(tag, "konto:closed:")); err == nil {
			d.Closed = date
		}
	}
}

// ClosedTagKey is the directive tag that records when an account was closed.
const ClosedTagKey = "konto:closed"

// RefusalError is why an account invariant forbids a change; its text is for the user.
type RefusalError struct{ Reason string }

func (r RefusalError) Error() string { return r.Reason }

// The refusals that mean "nothing to do" rather than "not allowed": the UI stays quiet about them.
var (
	ErrAlreadyClosed = RefusalError{"the account is already closed"}
	ErrNotClosed     = RefusalError{"the account is not closed"}
)

// IsZero reports whether every commodity balance is zero.
func (m MultiAmount) IsZero() bool {
	for _, amount := range m {
		if amount.Mantissa != 0 {
			return false
		}
	}
	return true
}

// Close returns the changeset that closes the account on the given day. It refuses the clearing
// account, an account that is already closed, one with a balance, and one a periodic rule still
// references (rules lists their descriptions).
func (a Account) Close(today time.Time, rules []string) (Changeset, error) {
	if a.Source == SourceClearing {
		return Changeset{}, RefusalError{"the transfer clearing account can never be closed"}
	}
	if a.IsClosed() {
		return Changeset{}, ErrAlreadyClosed
	}
	if !a.Balance.IsZero() {
		return Changeset{}, RefusalError{"balance is " + a.Balance.Format() + " — only a zero balance can be closed"}
	}
	if len(rules) > 0 {
		return Changeset{}, RefusalError{"still used by: " + strings.Join(rules, ", ")}
	}
	return Changeset{Edits: []Edit{SetAccountTag{
		Account: a.Name, Key: ClosedTagKey, Value: today.Format(time.DateOnly),
	}}}, nil
}

// Reopen returns the changeset that reopens a closed account.
func (a Account) Reopen() (Changeset, error) {
	if !a.IsClosed() {
		return Changeset{}, ErrNotClosed
	}
	return Changeset{Edits: []Edit{RemoveAccountTag{Account: a.Name, Key: ClosedTagKey}}}, nil
}

// NormalizeIBAN is an IBAN as konto compares it: upper case, without the spaces banks print.
func NormalizeIBAN(iban string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(iban), " ", ""))
}

// ImportRefusal is why the account cannot be the target of a CSV import, or nil when it can
// (spec §5.3): closed, manual and unclassified accounts and the clearing account are refused.
func (a Account) ImportRefusal() error {
	switch {
	case a.IsClosed():
		return RefusalError{"account closed — reopen with o"}
	case a.Source == SourceClearing:
		return RefusalError{a.Name + " is the transfer clearing account — it is never an import target"}
	case a.Source == SourceManual:
		return RefusalError{a.Name + " is a manual account — only import-fed accounts can be imported into"}
	case a.Source == SourceNone:
		return RefusalError{a.Name + " is unclassified (no konto:source tag) — it cannot be an import target"}
	}
	return nil
}

// FileEntry is one item of a directory listing.
type FileEntry struct {
	Name  string
	IsDir bool
}
