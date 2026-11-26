# 03 konto domain model and core types

Type: grilling
Status: resolved
Blocked by: 01

## Question

Define konto's domain model (writes `CONTEXT.md`, and an ADR if the read/write-model split
is a real hard-to-reverse trade-off):

- Where the boundary sits: thin DTOs that mirror `hledger` JSON output 1:1, or a richer
  konto-owned model that the gateway maps into? Different answers per concern (e.g. thin for
  balances, rich for the import-staging state that has no hledger equivalent)?
- The core nouns and their relationships: Journal, Account (+ the manual-vs-import-fed
  distinction — a hard product rule), Transaction, Posting, Amount, Commodity,
  PriceDirective, PeriodicTransaction, ImportRule, CsvRow, Check/CheckResult.
- How valuation mode (`native` | `market`) is represented — a mode flag passed to the
  gateway, or two pre-computed views held in memory?
- How multi-commodity amounts are modelled so "+ 2 cur" and market collapse both fall out
  naturally.
- Which concepts are konto's own with no hledger counterpart (import staging,
  `.latest.<file>` markers, rule match-stats) and therefore fully owned here.

Feeds **Go package and module layout**, **Bubble Tea model composition**, **CSV import
subsystem design**, **Test seams and fixture strategy**.

## Answer

Glossary: `CONTEXT.md`. ADRs: `docs/adr/0001-rich-domain-model-boundary.md`,
`docs/adr/0002-flat-account-model.md`.

### Boundary — rich model + mapping layer (ADR 0001)

The gateway decodes hledger JSON into konto-owned domain types; views never see hledger
shapes. Decoding is tolerant of unknown/optional fields and carries any version shims in one
place (per ticket 01: the JSON schema has drifted in minor ways).

### Core value types

- **`Amount`** — hand-rolled fixed-point: `{Commodity, Mantissa int64, Scale int32}`, an
  exact mirror of hledger's `decimalMantissa`/`decimalPlaces`. Zero external deps. konto
  implements only the arithmetic it needs (add, negate, compare, rescale, format).
  int64 headroom is ample for personal-finance sums; guard aggregation against overflow.
- **`Commodity`** — `{Symbol string, Style}` where `Style` is hledger's `astyle` (side,
  spacing, decimal mark, digit groups, precision), sourced from `commodity` directives or
  any `print -O json` amount. `Amount.Format()` honours it.
- **`MultiAmount []Amount`** — one entry per commodity, in hledger's order. Helpers:
  `Single() (Amount, bool)`, `ExtraCount() int` (= `len-1`, the "+ N cur"),
  `IsAllNegative() bool`. Market valuation collapses a MultiAmount to `len 1`.
- **`ValuationMode`** = `Native | Market`. A gateway **query parameter** — toggling `v`
  re-runs the relevant hledger call with/without `-V`. No valuation math in konto.

### Account

- **`[]Account` — flat list of postable accounts only.** No parent/aggregation entities
  (ADR 0002). Any indentation is a pure view concern; the Dashboard Accounts panel is a
  flat, non-indented list.
- **`Account.Source`** — optional `Manual | Import` (unset = unclassified). Read from a
  `konto:source:manual|import` tag on the journal's `account` directive. konto parses
  `account` directives from journal text (the tag also inherits onto that account's
  postings in `print -O json`, verified on 1.52.1; `accounts --directives` drops it).
- **Unclassified account** (postable, no tag): excluded from Entry and from import targets,
  surfaced as a konto **WARN** check issue. When such an account is first opened, konto
  prompts manual/import and writes the tag to the `account` directive (creating the
  directive if absent). That write is a "journal write" — feeds ticket 02 and the new
  periodic-subsystem ticket, which share the write path.

### Domain entities (mapped from hledger JSON)

- **`Transaction`** ← `print -O json`: `{Index, Date, Status, Code, Description, Comment,
  Tags, Source SourcePos, Postings []Posting, Uncategorized bool}`. `SourcePos{File, Line}`
  retained for jump-to-`$EDITOR`. `Uncategorized` computed (posting to a configurable
  catch-all account, or importer-marked unmatched).
- **`Posting`** — `{Account, Amounts MultiAmount, BalanceAssertion *Assertion, Status,
  Comment, Tags, Type}` (`Type` = Regular/Virtual/BalancedVirtual).
- **`Assertion`** — `{Amount, Inclusive bool, Total bool, Pos SourcePos}` (round-trips via
  `pbalanceassertion`).
- **`PriceDirective`** — `{Date, From Commodity, Price Amount}`, parsed from `hledger prices`
  text output.
- **`PeriodicRule`** — named here only; fields + pause mechanism + write path are the new
  **Periodic transaction subsystem design** ticket.
- **`CheckIssue`** — `{Check string, Severity ERR|WARN, Message, File string, Line, Col int}`.
  hledger reports all issues as `Error:` and stops at the first; konto runs checks
  individually and assigns severity by check name (parseable/autobalanced/assertions/
  balanced = ERR; commodities/accounts/ordereddates/uniqueleafnames/tags/recentassertions
  and konto's own unclassified-account check = WARN). **`CheckRun`** aggregates issues +
  clean/dirty for the `● clean` indicator.

### konto-only entities (no hledger counterpart)

`ImportSource`, `CsvRow`, `ImportRule`, `RuleMatchStats`, `Watermark` (`.latest.<file>`) —
detailed in **CSV import subsystem design**.

### `Ledger` — the loaded snapshot

No monolithic `Journal` type. `Ledger` = `{Meta LedgerMeta, Accounts []Account,
Transactions []Transaction, Prices []PriceDirective, Periodics []PeriodicRule,
Checks CheckRun}`, each field populated by its own gateway call and independently
refreshable. `LedgerMeta` = `{JournalPath string, TxnCount int, HledgerVersion string,
LoadedAt time.Time, Health}` — backs the tab-bar right-aligned chrome.
