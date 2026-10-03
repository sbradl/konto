package domain

import (
	"testing"
	"time"
)

func TestParseSource(t *testing.T) {
	for value, want := range map[string]Source{
		"manual": SourceManual, "import": SourceImport, "clearing": SourceClearing, "": SourceNone, "bank": SourceNone,
	} {
		if got := ParseSource(value); got != want {
			t.Errorf("ParseSource(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestAccountIsClosedFollowsTheClosedDate(t *testing.T) {
	if (Account{}).IsClosed() {
		t.Error("an account without a closed date is open")
	}
	if !(Account{Closed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}).IsClosed() {
		t.Error("an account with a closed date is closed")
	}
}

func TestOnlyAssetsAndLiabilitiesAreClassifiable(t *testing.T) {
	for name, want := range map[string]bool{
		"assets:checking": true, "liabilities:card": true,
		"expenses:rent": false, "income:salary": false, "equity:opening-balances": false, "assets": false,
	} {
		if got := (Account{Name: name}).IsClassifiable(); got != want {
			t.Errorf("%s: IsClassifiable() = %v, want %v", name, got, want)
		}
	}
}

func TestUnclassifiedIsAClassifiableAccountWithoutASource(t *testing.T) {
	for name, tc := range map[string]struct {
		account Account
		want    bool
	}{
		"asset without source":     {Account{Name: "assets:spare"}, true},
		"asset with source":        {Account{Name: "assets:spare", Source: SourceImport}, false},
		"category without source":  {Account{Name: "expenses:rent"}, false},
		"liability without source": {Account{Name: "liabilities:card"}, true},
	} {
		if got := tc.account.IsUnclassified(); got != tc.want {
			t.Errorf("%s: IsUnclassified() = %v, want %v", name, got, tc.want)
		}
	}
}

func TestDeclarationApplyTag(t *testing.T) {
	var d Declaration
	for _, tag := range []string{"konto:source:import", "konto:iban:DE12", "konto:closed:2026-03-01", "konto:format:postbank", ""} {
		d.ApplyTag(tag)
	}

	want := Declaration{Source: SourceImport, IBAN: "DE12", Closed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	if d != want {
		t.Errorf("declaration = %+v, want %+v", d, want)
	}
}

func TestDeclarationIgnoresAnInvalidClosedDate(t *testing.T) {
	var d Declaration
	d.ApplyTag("konto:closed:someday")

	if !d.Closed.IsZero() {
		t.Errorf("Closed = %v, want zero", d.Closed)
	}
}
