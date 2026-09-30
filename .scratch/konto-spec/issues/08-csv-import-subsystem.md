# 08 CSV import subsystem design

Type: grilling
Status: resolved
Blocked by: 02, 03, 14

## Question

The Import view is konto's largest owned write-path. Decide its design:

- CSV parsing: konto-owned, or lean on `hledger import` / hledger's CSV rules engine? What
  does hledger's `import` give us for free (ticket 01) vs what konto must do itself?
  Note: konto needs its own raw-CSV read regardless (ticket 01 — `import --dry-run` won't
  enumerate skipped rows, and the Import view's row table shows parsed rows + match state).
- **Multi-format support.** Primary target is **Postbank** CSV exports (samples via ticket
  14); the subsystem must be extensible to other banks. Decide the abstraction:
  - A **bank profile / format** concept, one per bank, carrying: raw-parse settings
    (delimiter — German banks use `;`; character encoding — often ISO-8859-1 / Windows-1252,
    not UTF-8; decimal comma; date format e.g. `DD.MM.YYYY`; header/preamble rows to skip;
    Postbank exports may have a metadata preamble and/or multiple CSV sections), the
    running-balance column, and the hledger `.rules` file template.
  - Single source of truth for column layout: does konto read it from the hledger `.rules`
    `fields`/`skip`/`date-format` directives, or from its own profile config, or both?
  - hledger's `.rules` engine already adapts arbitrary CSV → hledger fields — how much
    format handling is delegated to it vs owned by konto's reader for the preview.
  - Where profiles live (built-in Go + user-supplied config?), how a file is matched to a
    profile (filename glob? header signature? explicit per-import-source config?).
- Rule generation: how konto builds an `if <pattern> / account2 <target> / comment <text>`
  block for a selected row, where it writes it (`.rules` file discovery/location), and how
  it computes the match-count stats (matches in this file / across the journal / existing
  rule count / rows still unmatched after saving).
- Dedup: reading and writing the `.latest.<file>` marker, showing skipped rows as a legible
  list, and the manual override (`d` mark genuine dup / `u` undo and re-import).
- Balance assertions: deriving `= <balance>` on the last posting from the CSV's
  running-balance column — column detection, sign handling, currency.
- The posting-preview: generating the exact journal text that will be appended, and which
  file / include-chain it goes to.

Feeds the "journal writes beyond CSV import" fog and **Assemble docs/spec.md**.

## Answer

Grilled 2026-09-30 in two passes: architecture first, then the Postbank format once the
samples (ticket 14) arrived. Deviations from the handoff → ADR 0009.

### Split: hledger converts, konto owns the rest

- hledger turns CSV into transactions: `hledger print -f <csv> --rules-file <combined>
  -O json` (no write). Its rules engine owns delimiter, encoding, date/decimal format and
  category `if` matching.
- konto owns dedup, the categorisation gate, the pre-write check and the write.
  **`hledger import` and `.latest.<file>` are not used** — `.latest` is keyed by filename
  and bank downloads get a new name each time; `import` appends to the main journal and
  can't list skipped rows.
- User directive: start here; switch to a konto-owned reader only if hledger proves
  insufficient.

### Dedup — the journal is the watermark

An import-fed account only ever receives imported transactions, so per account:
D = date of its latest transaction. Rows **< D** skipped; rows **> D** new; rows **= D**
compared by date + amount + description as a **multiset** (2 identical coffees in the CSV
vs 1 in the journal → 1 new). Skipped rows shown as a list. No state file, no `d`/`u`
manual override.

### Account & bank format

- Account detected by **IBAN** from the CSV preamble, matched against a `konto:iban:<IBAN>`
  tag. First import (unknown IBAN): the user chooses from import-fed accounts and konto
  stores the tag.
- Bank format stored on the account: `konto:format:<bank>` tag on its `account`
  directive, asked on the first import into that account.
- Formats (the `.rules` preamble: `separator`, `encoding`, `skip`, `fields`, `date-format`,
  `decimal-mark`, …) are **built into konto via `go:embed`**; a new bank = code change.
  Nothing written next to the journal for formats.

### Categories & rules

- **One shared `categories.rules`** next to the journal, for all import accounts. At import
  konto passes a combined rules file (temp) that `include`s the embedded format +
  `categories.rules` and sets `account1` to the chosen account.
- Per `?` (unmatched) row, three choices:
  1. **New rule** — pattern pre-filled with the **full description** (deliberately narrow;
     escaped literal); optional shortening with a live match count.
  2. **Add to existing rule** — pick a rule (filterable), konto appends the row's
     description as another matcher line (hledger ORs `if` lines). This is how a rule
     "widens" when a payee's name changes (user's KMyMoney pain point).
  3. **Once** — category on this transaction only, no rule.
- A `✓` row can be overridden with **once** (rule unchanged).
- Each rule carries `comment konto-rule:<id>` → hledger tags the generated transaction,
  so the table shows which rule matched.
- **Overlap refused**: saving/extending a rule that would match rows another rule already
  categorises (in the current file) is rejected, naming the rule. konto tests patterns
  itself in Go (case-insensitive regexp over the description) — a contained exception to
  "konto doesn't reimplement hledger", used only for this check; hledger stays the
  authority for categorisation.
- **Category picker**: type-to-filter over all categories; those matching the amount's
  direction first (`expenses:*` for outflows, `income:*` for inflows), the other kind still
  selectable (refunds). Typing an unknown name offers "+ create" — prefix pre-filled from
  direction, `uniqueleafnames` checked, `account` directive written.
- **Classification scope** (clarifies ticket 03): only **accounts** (`assets:*`,
  `liabilities:*`) carry `konto:source:`; categories (`expenses:*`, `income:*`) and equity
  are never classified, never WARN, usable from both Entry and Import.

### Write

- Enter writes only when **no `?` rows remain** (no skip — ticket 15).
- **Pre-write check**: konto runs hledger over the journal + the candidate transactions
  (stdin), without touching the file. A failing balance assertion **blocks** the import,
  showing bank balance vs journal balance vs difference.
- Target file: the file holding the account's latest transaction; first import → main
  journal. Inserted at date position; new payees declared (ticket 15).
- `= <balance>` assertion on the last transaction from the CSV's running-balance column.

### First import — opening balance

On the first import into an account with no transactions, konto derives the opening
balance from the CSV (trailer balance − sum of all rows) and writes, dated the day before
the oldest row:

```
<date> opening balance
    <account>                <amt> = <amt>
    equity:opening-balances
```

shown in the preview for confirmation. If the bank format has no balance column, the user
enters the opening balance by hand first (import blocked with a hint until then). Side
effect: the per-import assertion also catches **gaps** (CSV starting after D with missing
rows) as a blocked import.

### View

Filepicker (Downloads, `.csv`) → account (+ format on first import) → row table (`✓` by
rule id / `?` needs category) + skipped-rows list → right column: category picker + rule
choice (new / add to existing / once) + posting preview → Enter. Left column is an info
block (file, account, format, D) — **no file list with status dots**.

### Postbank format (from samples, verified on hledger 1.52.1 with a synthetic file)

- UTF-8 **with BOM**, `;` separator, LF, no quoting. 18 columns.
- **Preamble**: 7 non-blank lines (title; account/IBAN/currency header + values; period
  `d.m.yyyy - d.m.yyyy`; "Letzter Kontostand"; pending-bookings note) + one blank line,
  then the header row. hledger `skip` does not count blank lines → `skip 7`.
- **Trailer**: `Kontostand;<period end>;;;<balance>;EUR` — skipped via `if ^Kontostand` /
  `skip`. Its balance is the **balance at period end** and is reliable (reconciled exactly
  across the two overlapping exports). The preamble's "Letzter Kontostand" differs from it
  and is **ignored**.
- **Columns**: Buchungstag, Wert, Umsatzart, Begünstigter / Auftraggeber, Verwendungszweck,
  IBAN / Kontonummer, BIC, Kundenreferenz, Mandatsreferenz, Gläubiger ID, Fremde Gebühren,
  Betrag, Abweichender Empfänger, Anzahl der Aufträge, Anzahl der Schecks, Soll, Haben,
  Währung. `Betrag` is signed; `Soll`/`Haben` redundant.
- Dates `d.m.yyyy` without zero padding (`%-d.%-m.%Y`); amounts German (`1.023,45`,
  `decimal-mark ,`); currency always EUR.
- **Newest first.** Booking and value date differ on ~5% of rows.
- **Pending bookings are not exported** ("nicht Bestandteil dieser Übersicht").
- No per-row running balance.
- Payee empty for `Auszahlung`, `Bargeldeinzahlung`, `Kontoabrechnung`. Payment providers
  (PayPal, Klarna) appear as payee; the merchant is only in the purpose text.
- Identical same-day rows (date + payee + amount) occur → multiset dedup confirmed.
- Overlapping period across two exports: rows identical, same order.
- Filename `Kontoumsaetze_<account number>_<timestamp>.csv` — changes per download.

### Journal mapping

- **Date**: booking date only (no secondary date).
- **Description**: payee; if empty, the transaction type (`Umsatzart`).
- **Transaction comment**: purpose text.
- **Rule id**: on the category posting via `comment2 konto-rule:<id>` (a transaction-level
  comment would overwrite the purpose); kept in the journal, `tag konto-rule` declared.
- Nothing else (type, counterparty IBAN, references) is written.

### Rules (refined)

- Match on payee, optionally AND a purpose condition — verified syntax:
  ```
  if %payee PayPal
  & %purpose Thalia
    account2 expenses:books
    comment2 konto-rule:7
  ```
  OR = further matcher lines / blocks. For empty-payee rows the rule matches the field that
  supplied the description (`%type`). "New rule" default: payee (or type) only.

### Balance

- `= <balance>` assertion on the **newest** transaction, from the trailer.
- Opening balance (first import) = trailer − sum of all rows.

### Fixtures

Synthetic Postbank-format CSVs under `testdata/` (invented names, IBANs, amounts; consistent
balances): single export, overlapping pair, identical same-day rows, empty-payee rows,
PayPal-with-purpose rows. Never copies of the real files (ticket 14).
