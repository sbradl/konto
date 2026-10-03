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
func (a Account) IsClassifiable() bool {
	return strings.HasPrefix(a.Name, "assets:") || strings.HasPrefix(a.Name, "liabilities:")
}

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
