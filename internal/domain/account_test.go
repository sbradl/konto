package domain

import (
	"errors"
	"reflect"
	"strings"
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

var today = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

func spare() Account {
	return Account{Name: "assets:spare", Source: SourceImport, Balance: MultiAmount{eur(0)}}
}

func TestCloseWritesTheClosedTagWithTodaysDate(t *testing.T) {
	got, err := spare().Close(today, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Changeset{Edits: []Edit{SetAccountTag{Account: "assets:spare", Key: "konto:closed", Value: "2026-09-30"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changeset = %+v, want %+v", got, want)
	}
}

func TestCloseIsRefusedWithTheReason(t *testing.T) {
	closed := spare()
	closed.Closed = today
	checking := spare()
	checking.Balance = MultiAmount{eur(1324000)}
	clearing := spare()
	clearing.Source = SourceClearing

	for name, tc := range map[string]struct {
		account Account
		rules   []string
		want    string
	}{
		"clearing":      {clearing, nil, "clearing"},
		"already":       {closed, nil, ErrAlreadyClosed.Reason},
		"balance":       {checking, nil, "balance is 13.240,00 EUR"},
		"negative":      {Account{Name: "liabilities:card", Balance: MultiAmount{eur(-14000)}}, nil, "balance is -140,00 EUR"},
		"rules":         {spare(), []string{"Top-up", "Alpha"}, "still used by: Top-up, Alpha"},
		"clearing wins": {Account{Name: "assets:transfers", Source: SourceClearing, Balance: MultiAmount{eur(5)}}, nil, "clearing"},
	} {
		_, err := tc.account.Close(today, tc.rules)

		var refusal RefusalError
		if !errors.As(err, &refusal) || !strings.Contains(refusal.Error(), tc.want) {
			t.Errorf("%s: error = %v, want a refusal containing %q", name, err, tc.want)
		}
	}
}

func TestAnAccountWithoutPostingsHasAZeroBalance(t *testing.T) {
	if _, err := (Account{Name: "assets:new"}).Close(today, nil); err != nil {
		t.Errorf("an account with no balance entries should close: %v", err)
	}
}

func TestReopenRemovesTheClosedTag(t *testing.T) {
	account := spare()
	account.Closed = today

	got, err := account.Reopen()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Changeset{Edits: []Edit{RemoveAccountTag{Account: "assets:spare", Key: "konto:closed"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changeset = %+v, want %+v", got, want)
	}
}

func TestReopenOfAnOpenAccountIsRefused(t *testing.T) {
	if _, err := spare().Reopen(); !errors.Is(err, ErrNotClosed) {
		t.Errorf("error = %v, want ErrNotClosed", err)
	}
}

func TestMultiAmountIsZero(t *testing.T) {
	for name, tc := range map[string]struct {
		amounts MultiAmount
		want    bool
	}{
		"empty": {nil, true}, "zero": {MultiAmount{eur(0)}, true}, "non-zero": {MultiAmount{eur(0), eur(1)}, false},
		"negative": {MultiAmount{eur(-1)}, false},
	} {
		if got := tc.amounts.IsZero(); got != tc.want {
			t.Errorf("%s: IsZero() = %v, want %v", name, got, tc.want)
		}
	}
}

func TestNormalizeIBANRemovesSpacesAndFoldsCase(t *testing.T) {
	for in, want := range map[string]string{
		"DE89370400440532013000":      "DE89370400440532013000",
		"DE89 3704 0044 0532 0130 00": "DE89370400440532013000",
		" de89370400440532013000 ":    "DE89370400440532013000",
	} {
		if got := NormalizeIBAN(in); got != want {
			t.Errorf("NormalizeIBAN(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImportRefusalNamesWhyAnAccountCannotBeImportedInto(t *testing.T) {
	closed := Account{Name: "assets:old", Source: SourceImport, Closed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	for name, tc := range map[string]struct {
		account Account
		want    string
	}{
		"closed":       {closed, "account closed — reopen with o"},
		"clearing":     {Account{Name: "assets:transfers", Source: SourceClearing}, "assets:transfers is the transfer clearing account"},
		"manual":       {Account{Name: "assets:cash", Source: SourceManual}, "assets:cash is a manual account"},
		"unclassified": {Account{Name: "assets:odd"}, "assets:odd is unclassified"},
	} {
		err := tc.account.ImportRefusal()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: refusal = %v, want it to say %q", name, err, tc.want)
		}
	}
	if err := (Account{Name: "assets:checking", Source: SourceImport}).ImportRefusal(); err != nil {
		t.Errorf("an open import-fed account is a target, got %v", err)
	}
}

func TestImportRefusalNamesWhyAnAccountIsNoImportTarget(t *testing.T) {
	closed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	for name, tc := range map[string]struct {
		account Account
		want    string
	}{
		"closed":       {Account{Name: "assets:a", Source: SourceImport, Closed: closed}, "account closed — reopen with o"},
		"clearing":     {Account{Name: "assets:a", Source: SourceClearing}, "assets:a is the transfer clearing account — it is never an import target"},
		"manual":       {Account{Name: "assets:a", Source: SourceManual}, "assets:a is a manual account — only import-fed accounts can be imported into"},
		"unclassified": {Account{Name: "assets:a"}, "assets:a is unclassified (no konto:source tag) — it cannot be an import target"},
		"import-fed":   {Account{Name: "assets:a", Source: SourceImport}, ""},
	} {
		err := tc.account.ImportRefusal()
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: unexpected refusal %v", name, err)
		case tc.want != "" && (err == nil || err.Error() != tc.want):
			t.Errorf("%s: refusal = %v, want %q", name, err, tc.want)
		}
	}
}
