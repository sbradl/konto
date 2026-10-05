# konto — foundation spec

konto is a terminal UI frontend for [hledger](https://hledger.org), single-user personal
finance. This document is the **settled architectural foundation**: the decisions a build
session needs before writing code. It does not restate the UX — that lives in
[`design_handoff_hledger_tui/README.md`](design_handoff_hledger_tui/README.md), which is
authoritative and frozen (except the deviations in §11).

Per-feature design (periodic rules, CSV import and forecast resolved into §4.8–§4.10;
responsive layout into §6.8) beyond that is **deliberately not in this spec** — it is grilled just-in-time as each view is built
(§12).

Sources: the wayfinder map and decision tickets under `.scratch/konto-spec/`, `CONTEXT.md`
(domain glossary), `docs/adr/0001`–`0013`.

---

## 1. Technology stack

| Concern | Choice | Pin |
|---|---|---|
| Language | Go | `go.mod` says `go 1.27` |
| TUI framework | Bubble Tea **v2** | `charm.land/bubbletea/v2 v2.0.9` |
| Styling | Lip Gloss **v2** | `charm.land/lipgloss/v2 v2.0.6` |
| Components | Bubbles **v2** | `charm.land/bubbles/v2 v2.2.1` |
| Charts | ntcharts **v2** | `github.com/NimbleMarkets/ntcharts/v2 v2.2.0` |
| Accounting engine | the `hledger` **binary** (hard runtime dependency) | require `>= 1.32` |

v2 uses vanity import paths (`charm.land/...`), **not** `github.com/charmbracelet/.../v2`.
v1 is frozen; do not use it. Rationale and the overlay-compositing consequence: ADR 0003 is
about layering; the stack choice is in ticket `06-overlay-popup-compositing` and
`research/06-overlay-compositing.md`.

**Stack locked** (ticket 07, 2026-09-30): `ntcharts/v2`'s `go.mod` carries an ignored
`replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 ...`; a trivial
`timeserieslinechart` program built cleanly against real `charm.land/bubbletea/v2 v2.0.10`
+ `lipgloss/v2 v2.0.6` + `ntcharts/v2 v2.2.0`. The Charm v1 fallback is off the table.

No external decimal, config, or CSV-framework dependency is planned. Keep the dependency
tree minimal.

---

## 2. Architecture — ports and adapters (ADR 0003)

Clean Architecture (Martin). Every source dependency points **inward** toward
`internal/domain`. The one boundary that would otherwise invert — a use case needing an
adapter — is bridged by a **consumer-side port**: the use case declares the interface it
needs; the adapter satisfies it *structurally* (Go implicit interfaces); `cmd/konto` wires
the concrete in.

```
cmd/konto            Composition root. Only package importing concretes from every circle.

internal/ui/…         Frameworks & Drivers. Bubble Tea / Lip Gloss confined here.
internal/{ledger,accounts,checks,report,csvimport,periodic,forecast,entry}
                     Use cases. Plain Go, no tea/lipgloss/exec. Each declares its own ports.
internal/hledger     Adapter → the hledger binary.       ┐ import domain only;
internal/journal     Adapter → the journal text file.    ┘ satisfy the ports structurally
internal/domain      Entities. Types + pure rules. Imports nothing of konto's.
```

The port pattern, concretely:

```go
// internal/report
type ledgerGateway interface {
    BalanceSheet(p domain.Period, v domain.ValuationMode) (domain.ReportTable, error)
    IncomeStatement(p domain.Period, v domain.ValuationMode) (domain.ReportTable, error)
}
type Service struct { gw ledgerGateway }
func New(gw ledgerGateway) *Service { return &Service{gw} }
```

`*hledger.Client` already has methods with those signatures, so `report` imports `domain`,
`hledger` imports `domain`, and neither imports the other. `cmd/konto`:

```go
hl      := hledger.New(cfg.HledgerBin, cfg.JournalPath)
reports := report.New(hl)     // *hledger.Client used as report.ledgerGateway
```

**Deliberate deviations from strict Clean Architecture** (ADR 0001, 0003): adapters return
`domain` entities rather than per-use-case response models (`domain` *is* the inner form);
no standalone presenter (view packages format `domain` → cells themselves — fine for one
TUI); `domain` is thin because konto is a frontend and the accounting rules live in hledger.

---

## 3. Package tree

```
cmd/konto/main.go        wiring: config → hledger version preflight → construct adapters →
                         construct use-case services → construct root ui model → tea.Program

internal/
  domain/                SHARED KERNEL (ADR 0012). types + pure rules (Amount arithmetic/
                         format, MultiAmount helpers, ReportRow.Kind, check-severity-by-name,
                         Account aggregate behaviour, Changeset/Edit write model). No I/O.

  hledger/               ADAPTER. unexported run(ctx, args...) exec chokepoint + exported
                         Client (Balance, Print, Register, ARegister, Prices, Check,
                         ConvertCSV, Version). hledger JSON/text ⇄ domain. No tea import.
  journal/               ADAPTER. targeted line reader of journal text (~ blocks, account
                         directives + tags, categories.rules) + Apply(Changeset). NOT a parser.

  ledger/                USE CASE. Loader + the Ledger snapshot type; load/refresh by
                         composing hledger + journal. Read-only.
  accounts/              USE CASE. close/reopen (ADR 0012).
  checks/                USE CASE. the check run: per-check hledger calls + konto's own
                         checks → CheckRun (ADR 0012).
  report/                USE CASE. Reports: bs/is for the selected period (native only in v1).
  csvimport/             USE CASE. import staging, dedup, rule building, write. Owns
                         StagedImport, CsvRow, ImportRule, Watermark. (t.08)
  periodic/              USE CASE. ~ rule read/new/edit/delete. Owns RuleSpec. (ticket 13)
  forecast/              USE CASE. read-only projection. (ticket 09)
  entry/                 USE CASE. draft-entry validation, commit. NOT IN v1 (ADR 0010).
  config/                journal-path resolution (no config file in v1).

  ui/                    ui.go = root model, tab chrome, app-level keymap dispatch, wiring.
    theme/               ANSI-16 palette → named lipgloss.Style values.
    layout/              responsive layout helpers + the lipgloss.Compositor overlay wrapper.
    keys/                key.Binding sets (global + per view); the per-view HINTS strings.
    widgets/             shared render helpers (stat card, panel frame, selectable-list
                         styling, ntcharts wrapper).
    dashboard/ imports/ scheduled/ forecast/ reports/   (entry/ deferred, ADR 0010)
                         one package per view. Imports domain + the use-case service(s) it
                         needs + the ui leaves. NEVER imports hledger/journal. NEVER imports
                         another view.
```

Naming: the Import view package is `imports` (avoids shadowing `import`); its use case stays
`csvimport`. One app/view name clash (`forecast`; `entry` too once it returns) — the view aliases its service
import (`forecastsvc "…/internal/forecast"`).

---

## 4. hledger integration

Full command-by-command detail: `.scratch/konto-spec/research/01-hledger-output.md`.
Verified against `hledger 1.52.1`.

### 4.1 Which commands, which output

- **`-O json`** for `print`, `register`, `aregister`, `balance`/`bal`, `balancesheet`/`bs`,
  `incomestatement`/`is`, `cashflow`/`cf`. This is the main path.
- **No machine format** for `check`, `accounts`, `commodities`, `prices` — text only.
- JSON amount quantities: use `decimalMantissa` + `decimalPlaces` (exact). Never
  `floatingPoint` (lossy).
- `bal -O json` keeps multi-commodity amounts separate (the "+ N cur" case); `-V` /
  `--value=end` collapses them to one commodity via the latest `P` directive.
- `bs`/`is` JSON is a compound balance report; subreports `Assets`/`Liabilities` and
  `Revenues`/`Expenses`.

### 4.2 What hledger will not give us → `internal/journal`

The `internal/journal` adapter is a **targeted line-oriented reader/writer, not a parser**
(a full parser port is out of scope). It handles only:

- **`~` periodic-rule blocks** — `print` drops `~` rules entirely; `print --forecast`
  expands them (generated txns tagged `_generated-transaction`, value = interval text,
  `tsourcepos` = the `~` line). No command lists the rules. konto reads the `~` line +
  indented postings + trailing comment/tags from journal text. Dates are never computed
  from this text — see §4.8.
- **`account` directives and their tags** — the `konto:source:manual|import|clearing` tag
  (§7), the `konto:closed:<date>` tag (§5.3) and the `konto:format:<bank>` /
  `konto:iban:<IBAN>` tags (§4.9). `accounts --directives` drops tags; the tag *does* inherit onto that account's
  postings in `print -O json`, but text parsing is the primary source.
- **`categories.rules`** — append new `if` blocks, append a matcher line to an existing
  block (§4.9). No `.latest` files (ADR 0009).
- **Writes** — one entry point, `Apply(domain.Changeset)` (ADR 0012). A `Changeset` is a
  list of `Edit`s: insert a transaction at its date position, add/modify/remove an
  `account`-directive tag (creating the directive if absent), declare an account/payee,
  append / replace-in-place / delete a `~` rule block (§4.8), append a `categories.rules`
  block or matcher line. Applied per file via temp + rename. Import, periodic rules and
  close/reopen all write through it; each declares its own
  `journalWriter{ Apply(domain.Changeset) error }` port.

It gets the loaded file list from `hledger files` (the main journal plus the window's year files,
ADR 0013) and works per file. It never needs to understand transactions, amounts, or include resolution.

### 4.3 Invocation architecture

- **`internal/hledger` is plain blocking Go**, no `bubbletea` import. `Client.Balance(q)
  (domain.ReportTable, error)` etc. The UI wraps each call in a `tea.Cmd` goroutine.
- Two layers inside it: unexported `run(ctx, args...) ([]byte, error)` — the single
  `exec.Command` chokepoint and the test seam — and exported `Client` that builds args,
  calls `run`, maps output to `domain`.
- `run` takes a `context.Context`; the UI adapter cancels an in-flight refresh superseded by
  a newer one.
- A **`Loader`** (`internal/ledger`) composes the `hledger.Client` + `journal.Reader` ports
  and produces a `Ledger` snapshot. The UI wraps `Loader.Load()` / `Loader.Refresh()` in a
  `tea.Cmd`.

### 4.4 Loading / caching

- **Eager**: `Loader` builds the full `Ledger` snapshot at startup; konto holds it. hledger
  calls are sub-100ms for a personal journal.
- **Refresh triggers**: explicit `r` key; automatically after any konto journal write;
  automatically on return from `$EDITOR`. **No filesystem watcher in v1.** All three go
  through one root path: a successful write makes the writing view emit
  `journalChangedMsg{summary}`; the root shows the status-bar confirmation, runs
  `Loader.Refresh()` + the check run, and broadcasts the results (ADR 0012). Views and write
  use cases never hold the `Loader`.
- **Granularity**: any journal write → **full `Ledger` refresh**. A `categories.rules` change
  on disk (konto's `w`, or a hand edit + `r`) does not touch the journal — it re-converts a
  staged import only.
- **In-flight UI contract**: stale-while-revalidate. The held snapshot always renders; a
  refresh sets an in-flight flag shown as a status-bar spinner; panels never blank. First
  load shows a centred "loading journal…". A failed refresh keeps the stale snapshot.

### 4.5 Errors and preflight

| Situation | Behaviour |
|---|---|
| `hledger` missing on `PATH`, or version `< 1.32` | **Refuse to start.** Preflight in `main` before the TUI: `exec.LookPath` → `hledger --version` → parse `hledger X.Y(.Z)` → compare. Print problem + minimum to stderr, exit non-zero. Version also stored in `LedgerMeta`. |
| No journal file configured / file absent | **First-run screen**: shows the resolved path (§7.1); `y` creates the skeleton there (parent dirs too), `n` quits. No path input — another location is chosen via `$LEDGER_FILE` / `--file` (no config file). Skeleton: `decimal-mark ,`, `commodity 1.000,00 EUR`, `account equity:opening-balances`, `account assets:transfers  ; konto:source:clearing` (transfer clearing account), `tag konto:source` / `konto:iban` / `konto:format` / `konto:closed` / `konto-rule`; plus an empty `categories.rules` beside it. Check-clean from the first write. |
| Journal exists but `hledger` cannot parse it | Show the parse error as a top-severity `CheckIssue`; offer jump-to-`$EDITOR`; do not enter the normal views until it parses. |
| Runtime `hledger` call fails / unparseable output | Keep the last good `Ledger`; error indicator in the status bar + detail in the `!` panel; never crash. |

### 4.6 Journal checks (`!` panel)

hledger's `check` reports only the **first** failure per invocation and has **no
severities**. konto:

- runs a fixed set of checks, **each as its own `hledger check <name>` invocation**; parses
  the first-failure message from stderr into a `CheckIssue` (regex the `file:line[:col]`
  prefix). Each check contributes 0 or 1 issue → the panel shows ≤ ~10 rows.
- **Fix-one → refresh → next-instance-appears** is the working loop (hledger cannot do
  better). Document this in-app.
- Check set: **every hledger check, always, not configurable** (ticket 15) — basic
  (`parseable`, `autobalanced`, `assertions`), strict (`balanced`, `commodities`,
  `accounts`), plus `ordereddates`, `tags`, `payees`, `recentassertions`, `uniqueleafnames`.
- konto's own writes must pass them: insert transactions at their **date position** in the
  file (not append); auto-declare konto's tags (`tag konto:source`, …), each new payee, and
  new categories/accounts; refuse a new name whose last part already exists
  (`uniqueleafnames`). `recentassertions` is satisfied by import's `=` assertion.
- konto's own checks (unclassified account, and future ones) run in Go over the `Ledger` and
  **enumerate fully**.
- Severity assigned by check name (§7 `CheckIssue`).
- `parseable` / `autobalanced` / `assertions` fail at **load** (konto does not pass `-I`): they surface as
  the parse-error screen (§4.5), never as `!` rows. The `!` panel therefore only ever lists the WARN checks
  plus konto's own.

### 4.7 `$EDITOR`

`tea.ExecProcess` (Bubble Tea v2) suspends the TUI, runs `$VISUAL` → `$EDITOR` → a sane
default with `+<line> <file>`, resumes, then fires a full refresh. Used by the `!` panel's
jump-to-location — its only use in v1 (no `e`-to-editor on list views, ADR 0011).

### 4.8 Periodic rules (ticket 13, ADR 0006)

- **Forecast-only.** konto never writes an occurrence; no Enter-to-post, no pause.
- **Dates from hledger only**: one `print --forecast=<today>..<today+1y> -O json`, generated
  txns grouped by `tsourcepos` (the `~` line) → next date, next 6 occurrences, 12-month sum.
  `--forecast=PERIOD` must be explicit (default horizon starts after the latest regular
  transaction, not today). Status = upcoming / none in the next 12 months. Cadence column =
  raw period expression.
- **Form** (`n`, `e`): next due date, frequency dropdown (`once`/`weekly`/`biweekly`/
  `monthly`/`quarterly`/`half-yearly`→`every 6 months`/`yearly`), optional remaining-payment
  count (hidden for `once`), description, amount + currency (declared commodities), from/to
  account (existing, open only). Writes `~ <freq> from <date>[ to <end>]  <desc>` (or
  `~ <date>  <desc>` for `once`) + two postings, amount on the first.
  Count → `to <N-th occurrence + 1 day>`, N-th date obtained by piping the candidate rule to
  `hledger -f - print --forecast=…`.
- **`e`** only on rules that fit the form (that expression shape, 2 postings, one
  single-commodity amount without `@`/`=`, no comments/tags); otherwise refused. Save
  replaces the block in place.
- **`n`** appends to the main journal (rules live there, ADR 0013).
- **`d`** deletes any rule after y/n (handoff's `x`, renamed per §6.4): the `~` line + its indented lines; comments above stay.
- **Write safety**: full reload after every write; single-instance `flock` on a lock file
  next to the journal at startup (a second konto refuses to start). No compare-before-write.
- Annual impact per rule = signed 12-month change in asset/liability balances (expense red, income green,
  transfer between own accounts `0,00` dim); the total is the net change of net worth. Plain EUR
  (no valuation toggle in v1, ADR 0011).

### 4.9 CSV import (ticket 08, ADR 0009)

- **hledger converts, konto owns the rest.** `hledger print -f <csv> --rules-file <tmp>
  -O json`, where `<tmp>` `include`s the embedded bank format (`go:embed`) +
  `categories.rules` (one shared file next to the journal) + the **staged** rules and sets
  `account1`. No `hledger import`, no `.latest`.
- **Staged import**: everything the flow creates — new account, new categories, new/widened
  rules, row categorisations — is held in memory and written as **one unit on `w`** (one
  y/n, one check run). Nothing touches disk before `w`. Each staged-rule change re-converts
  the CSV through the temp rules file, so hledger still applies all rules.
- **Flow**: filepicker (XDG Downloads, `.csv`) → account detected by IBAN from the preamble
  (`konto:iban:<IBAN>`; unknown IBAN → **create account**: suggested, editable name,
  `uniqueleafnames`-checked, `account` directive with `konto:source:import`, `konto:iban:`,
  `konto:format:<bank>`, staged for the main journal; no attach-to-existing, ADR 0010) → row table (`✓` rule id / `?`) + skipped list →
  categorise → Enter.
- **Dedup**: journal is the watermark — D = the account's latest transaction date; `< D`
  skipped, `> D` new, `= D` compared by date+amount+description as a multiset.
- **Categorise** each `?` row: new rule (pattern = full description, optional shortening,
  live count) / add to existing rule (appends an OR matcher line) / once. `✓` rows may be
  overridden once. Rules carry `comment konto-rule:<id>`. **Overlapping rules refused**
  (konto matches patterns in Go, case-insensitive, for this check only).
- **Category picker**: all categories, amount's direction first, then the other open
  accounts as **transfer** targets (ADR 0008 amendment) — an import-fed target is replaced by
  the **transfer clearing account**, so each side imports its own leg and no import-fed
  account gets a transaction from another account's CSV (keeps ADR 0009's watermark sound); "+ create" (prefix from
  direction, `uniqueleafnames` check, `account` directive).
- **Write** (`w`): only when no `?` rows remain; writes the account directive (if new),
  category directives, `categories.rules` changes and transactions together; pre-check journal + candidates via hledger on
  stdin — a failing `= <balance>` assertion blocks with bank / journal / difference.
  Each transaction goes to the year file of its own date (created on first write; ADR 0013),
  date-ordered insert, payees declared. A crossing into a new year also writes the carry; rows
  older than last year go to their own year files, unloaded, with the carry chain between them.
- **After the write**: status-bar confirmation (`imported N transactions into <account>`),
  then Import returns to its empty start state (`n` picks the next file). The staged import
  survives view switches (views stay alive, §6.1); `q` / `Ctrl+C` with a staged import asks y/n before
  quitting — discarding it leaves disk untouched.
- **First import** into an empty account: konto derives an opening balance from the CSV
  (trailer balance − sum of rows), writes it against `equity:opening-balances` with an
  assertion, dated the day before the oldest row; confirmed in the preview. Written even
  when it is zero (uniform: every import-fed account starts with one).
- **Postbank** (only bank at v1): UTF-8+BOM, `;`, `skip 7`, trailer `Kontostand;…` skipped via
  `if ^Kontostand`; `%-d.%-m.%Y`; `decimal-mark ,`; newest first; no pending rows; no
  per-row balance — the trailer is the period-end balance → `=` assertion on the newest
  transaction. Mapping: booking date only; description = payee (else `Umsatzart`);
  transaction comment = purpose; rule id as posting tag `konto-rule:<id>` (`comment2`,
  kept, declared). Rules match payee, optionally `& %purpose …` (PayPal/Klarna). Nothing
  else written. Full format notes: ticket 08.

### 4.10 Forecast (ticket 09, ADR 0007)

- **Current balances + scheduled transactions only.** No averages, no scenarios, no drivers.
- **Projection starts at each account's watermark** (ADR 0007 amendment), not today:
  occurrences between the last import and today count, so a stale import doesn't understate
  the forecast. Import-fed: from watermark + 1 day; manual (no watermark): from today.
- **One call per account**: `bal <account> -M -H --layout=bare -b <today−6m>
  --forecast=<start>..<today+12m> -O json` → month-end balance per commodity, history +
  forecast (one global `--forecast` start would double-count or miss occurrences across
  accounts with different watermarks). konto drops closed accounts and the clearing account,
  groups by commodity, ignores `Total:`.
- **View**: one braille chart per currency, stacked vertically; one line per open
  asset/liability account in that currency; colour per account in fixed order (ANSI
  2,4,5,6,3,1); legend; dim vertical line at today. Below: milestone table
  (accounts grouped by currency × today/+3/+6/+12 months), no totals.

---

## 5. Domain model

Glossary: `CONTEXT.md`. Full type sketch: ticket `03-domain-model-core-types`.

**Where types live** (ADR 0012): `internal/domain` is the **shared kernel** — the types
more than one context uses (§5.1–§5.3, §5.6). A type only one context uses lives in that
use-case package (§5.4). "Today" is never read from the system clock inside a use case:
each service that needs it takes `today func() domain.Date` in its constructor.

### 5.1 Value types (`internal/domain`)

- **`Amount`** — `{Commodity, Mantissa int64, Scale int32}`. Exact mirror of hledger's wire
  form. Hand-rolled arithmetic (add, negate, compare, rescale, format); guard aggregation
  against int64 overflow. No external decimal library.
- **`Commodity`** — `{Symbol string, Style}`; `Style` = hledger's `astyle` (side, spacing,
  decimal mark, digit groups, precision). `Amount.Format()` honours it.
- **`MultiAmount []Amount`** — one entry per commodity, hledger's order. `Single() (Amount,
  bool)`, `ExtraCount() int` (= `len-1`), `IsAllNegative() bool`. Market valuation → `len 1`.
- **`ValuationMode`** = `Native | Market`. A gateway query parameter (§4). No valuation math
  in konto. **Not built in v1** — the UI is EUR-only (ADR 0011); every call is native.

### 5.2 Entities (mapped from hledger by the adapter)

- **`Transaction`** `{Index, Date, Status, Code, Description, Comment, Tags, Source
  SourcePos, Postings []Posting}`. `SourcePos{File, Line}` for jump-to-`$EDITOR`. No
  `Uncategorized` — every transaction is categorised (ADR 0008).
- **`Posting`** `{Account, Amounts MultiAmount, BalanceAssertion *Assertion, Status,
  Comment, Tags, Type}` (`Type` = Regular/Virtual/BalancedVirtual).
- **`Assertion`** `{Amount, Inclusive bool, Total bool, Pos SourcePos}`.
- **`PriceDirective`** `{Date, From Commodity, Price Amount}` (from `hledger prices` text).
- **`PeriodicRule`** `{Period string (raw expression), Description, Postings []Posting,
  Raw string (block text), Pos SourcePos, FitsForm bool}`. `FitsForm` is computed (§4.8).
  Occurrences / annual impact are joined from the forecast call, not stored on the rule.
- **`CheckIssue`** `{Check string, Severity ERR|WARN, Message, File string, Line, Col int}`.
  `CheckRun` aggregates issues + clean/dirty for the `● clean` indicator. Severity by check
  name: `parseable`/`autobalanced`/`assertions`/`balanced` = ERR; `commodities`/`accounts`/
  `ordereddates`/`uniqueleafnames`/`tags`/`payees`/`recentassertions` + konto's
  unclassified-account check = WARN.
- **`ReportTable`** `{Rows []ReportRow}` where `ReportRow{Kind, Label, Depth, Amounts
  []MultiAmount, Delta *MultiAmount}` (no `Share` — dropped, ADR 0011), `Kind ∈ {SectionHeader, Leaf,
  NegativeLeaf, Subtotal, GrandTotal}` computed from structure + sign.

### 5.3 Account (ADR 0002)

- **`[]Account` — flat list of postable accounts only.** No parent/aggregation entities.
- **`Account.Source`** — optional `Manual | Import | Clearing`, unset = unclassified. `Clearing` = the one transfer clearing account `assets:transfers`: in net worth and the account list, never closable, never an import target, not forecast. **Only
  `assets:*` / `liabilities:*`** are classified; categories (`expenses:*`, `income:*`) and
  equity never are, never WARN, and are usable from Import (and Entry, once it returns). From the
  `konto:source:` tag on the `account` directive (§4.2).
- **Unclassified** (asset/liability, no tag): excluded from import targets; a konto WARN.
  No classify prompt (ADR 0010): konto only runs on journals it created and every account is
  created by first import, so the WARN is a safety net for hand edits in `$EDITOR`.
- **Manual** accounts stay in the model but have no v1 UI (ADR 0010); import refuses them.
- **`Account.Closed`** — optional date, from a `konto:closed:<date>` tag. Closing is refused
  unless balance is zero and no `~` rule references the account. Closed accounts appear only
  in the Dashboard Accounts panel; excluded from Forecast, Entry, import targets and the rule form.
- **Close / reopen** (Dashboard Accounts panel): **`c`** closes the selected open account after
  y/n — refused with the reason (nonzero balance / referencing rules) — writing
  `konto:closed:<today>`; **`o`** reopens a closed one after y/n (removes the tag). Closed
  accounts list dimmed (`8`) below the open ones with `closed <date>` in the balance column,
  counted in no total or stat card. Import into a closed account's IBAN is refused:
  "account closed — reopen with `o`".
- **`Account` is an aggregate** (ADR 0012): `Close(balance, referencingRules)`, `Reopen()`
  and `CanBeImportTarget()` enforce the rules above and return the `Changeset` to write.
  `internal/accounts` orchestrates close/reopen (loads balance + referencing rules, calls the
  aggregate, validates, applies).
- Balance-report hierarchies are `ReportTable`, separate from `[]Account`.

### 5.4 konto-only entities (no hledger counterpart)

Context-owned, in their use-case package (ADR 0012):

- **`internal/csvimport`** — `StagedImport` (aggregate root, in-memory only: target
  account or new account, rows, staged rules, new categories, opening balance, closing
  assertion; `Categorise`, `AddRule` refusing overlap, `CanWrite`, `Changeset()`), `CsvRow`
  (skipped / matched / needs category / once), `ImportRule`, `RuleMatchStats`, and
  `Watermark` — a value object derived from the journal on each import, never stored
  (ADR 0009). No `ImportSource`: the source is the chosen file. Ticket 08 (§4.9).
- **`internal/periodic`** — `RuleSpec`, the form's value object (frequency, from, count,
  amount, from/to account) that renders a `~` block (§4.8).
- Forecast has none — `ForecastDriver` / `Scenario` dropped, ADR 0007.

### 5.5 `Ledger` — the loaded snapshot (`internal/ledger`)

No monolithic `Journal` type. `Ledger` `{Meta LedgerMeta, Accounts []Account, Transactions
[]Transaction, Prices []PriceDirective, Periodics []PeriodicRule}`; each
field populated by its own gateway call, independently refreshable. `LedgerMeta`
`{JournalPath, TxnCount, HledgerVersion, LoadedAt, Health}` — backs the tab-bar chrome.
Views hold a read-only `*Ledger`, replaced wholesale on refresh, never mutated. The
`CheckRun` is not part of it: `internal/checks` produces it and the root holds the latest
one (ADR 0012).

### 5.6 `Changeset` — the write model (`internal/domain`, ADR 0012)

`Changeset{Edits []Edit}`, pure data. Edits: insert transaction (date position), set /
remove account-directive tag, declare account / payee, append / replace / delete a `~`
block, append a `categories.rules` block or matcher line. Produced by `Account`,
`StagedImport` and the `periodic` service; applied by `internal/journal` (§4.2). Every write
runs **validate → apply → refresh**: pre-check via hledger on stdin, `Apply`, then
`journalChangedMsg` (§4.4).

---

## 6. Bubble Tea composition (ADR 0004)

Full detail: ticket `05-bubble-tea-model-composition`.

### 6.1 Root model (`internal/ui`)

The program's single `tea.Model`. Holds:

- `appMode ∈ {firstRun, parseError, normal}` — first two render their own screen, not the
  tab UI, not a seventh view.
- the **five view sub-models** (six once Entry returns, ADR 0010), constructed at startup, alive for the whole run. Tab switch
  changes which one renders + receives input; cursor/scroll/draft state survives.
- **app-global state**: terminal size, the read-only `*domain.Ledger`, the latest
  `CheckRun`.
- **chrome**: active view id, tab bar, status bar. **overlay state**: `keysPopupOpen` (`?`),
  `checksPanelOpen` (`!`). No which-key goto, no command line (ADR 0011).
- the use-case **ports** it needs (`ledger` Loader, `checks` run) — injected by `cmd/konto`.
  It handles `journalChangedMsg` (§4.4).

Views never reference each other. Cross-view effects are mediated: a view acts → state
changes → root broadcasts → every view updates.

### 6.2 View sub-model interface

```go
type View interface {
    Update(tea.Msg) (View, tea.Cmd)
    View() string          // renders into its content area (size via WindowSizeMsg)
    Title() string
    StatusHint() string    // handoff HINTS map
    KeyMap() []key.Binding  // feeds the ? popup
}
```

Each `Update` is a pure `(Msg) → (View, Cmd)` — unit-testable with no running program. A
view holds the use-case port(s) it needs + its own view-local state (cursor, form field,
scroll, generation counter).

### 6.3 Message routing (root's `Update`)

| Message class | Routed to |
|---|---|
| `tea.KeyMsg`, `tea.MouseMsg` | open overlay if any, else the **active view only**. Global keys (`1`–`5`, `?`, `!`, `r`, `q`/`ctrl+c`) caught by root **first**; if the active view has a focused text input, only `Esc`/`Ctrl+C` are caught. |
| `tea.WindowSizeMsg` | root computes the content rect (window − tab bar − rule − status bar), forwards a **content-dimension** `WindowSizeMsg` to **all** views |
| `ledgerLoadedMsg`, `checksLoadedMsg`, ticks | root updates its state, **broadcasts** to all views |
| view-specific async results (`reportLoadedMsg`, …) | root forwards to the **one owning view** |

Shared-state changes are **messages, not shared pointers**. Async races: each request
carries a per-view monotonic generation number; stale results are dropped (pairs with §4.3
context cancellation). Sub-model `Cmd`s bubble up and are `tea.Batch`-ed by root.

### 6.4 Keymaps (`internal/ui/keys`)

`key.Binding` sets — one global + one per view. Root resolves against the global set first
(after overlay / text-input checks), else forwards to the active view. The `?` popup renders
from the binding sets (`bubbles/help`).

**Key principle:** every key is the **first letter of its action** where possible, and a
letter means the same action in every view (`d` = delete, `n` new, `e` edit, `c` close,
`o` open). Every write triggered by a key asks y/n first.

**v1 keymap** (ADR 0011):

| Scope | Keys |
|---|---|
| Global | `1`–`5` views · `?` keys popup · `!` checks panel · `r` refresh · `q` / `Ctrl+C` quit · `Esc` cancel / close popup |
| Movement | `↑`/`↓` row · `PgUp`/`PgDn` page · `Home`/`End` ends · `Tab`/`Shift+Tab` form fields. No `j`/`k`. |
| Dashboard | Accounts panel: `c` close · `o` reopen |
| Import | `Enter` categorise row (popup: new rule / add to rule / once — `✓` rows: once only; picked with arrows + Enter) · `w` write import (y/n, only with no `?` rows) · `n` new file |
| Scheduled | `n` new · `e` edit · `d` delete |
| Reports | `p` cycle report period (Reports-local state; Dashboard is always month to date). Read-only, no cursor; arrows scroll both tables together. |
| `!` panel | arrows select · `Enter` jump to `$EDITOR` |

No panel focus: each view has at most one cursor panel; read-only panels that overflow show
`… N more`.

### 6.4a Chrome and Dashboard (v1)

- **Tab bar**: ` 1 dash │ 2 import │ 3 sched │ 4 forecast │ 5 reports `; right-aligned
  journal path + transaction count.
- **Status bar**: view name · per-view hint · `n/total` (cursor views only) · refresh
  spinner / error indicator · check status `● clean` (green) or `● 2 err · 3 warn` (red with
  errors, amber warnings-only) · `? keys`. No mode badge.
- **Dashboard**: two stat cards — `NET WORTH` (sum of open assets + liabilities) and
  `<MONTH> NET INCOME` (calendar month to date, income − expenses from `is`, note
  `<in> in / <out> out`, green/red). Left column: Accounts, full height. Right: Recent
  (newest first, all accounts, not selectable), Upcoming (next occurrences, not selectable),
  monthly spend chart (total `expenses:*` per month, last 6 complete months, one line).
- **`!` panel**: issues only; when clean a single dim-green `✓ all N checks pass`.
- **Reports**: `p` cycles `month to date · quarter to date · year to date · all time`, all
  ending today. Balance sheet = balance at today (`bs -H`) + **`Δ period`** = change within
  the period (second call, `bs --change -p <period>`), green/red, `0` dim; the Δ column is
  hidden under `all time`. Income statement = account + amount for the period only — no
  share, no third column.

### 6.5 Overlays

Root-owned. The `?` keys popup and the `!` checks panel are
`lipgloss.Compositor` (Lip Gloss v2) layers over the composed base:

- base view = one `Layer` at `X(0).Y(0).Z(0)` sized to `tea.WindowSizeMsg`
- the single visible popup = a `Layer` at `Z(10)`, `Y(screenH - popupH - 1)`
- `compositor.Render()` → `tea.View.Content`; cursor via `tea.View.Cursor` at the
  post-composite position

Only one popup is ever visible, so no z-stack management. Views know nothing about overlays.
Details: `research/06-overlay-compositing.md`.

### 6.6 Components

| Component | Use |
|---|---|
| `bubbles/table` | Import CSV-row grid, Reports tables |
| custom list widget (`ui/widgets`) | Dashboard panels (Accounts selectable; Recent, Upcoming read-only) |
| `bubbles/textinput` | rule-editor target, import account-name, category typeahead, form fields |
| `bubbles/viewport` | scrollable panels (journal preview, checks body) |
| `bubbles/key`, `bubbles/help`, `bubbles/spinner` | bindings, `?` render, refresh indicator |
| `ntcharts` | Dashboard trend + Forecast braille charts |
| `bubbles/list` | evaluated, **not used** |

ntcharts API fit (ticket 07): `timeserieslinechart` has named datasets (`PushDataSet`,
`SetDataSetStyle`, `DrawBrailleDataSets`) for one line per account, and
`WithYLabelFormatter` for thousands labels. Visual check in the shell/Forecast build: the
dim forecast-start vertical line, and colour where two series share a braille cell (one
colour per cell).

### 6.7 Startup

`cmd/konto`: config → hledger version preflight → construct adapters/services → construct
root model (inject ports + config) → `tea.NewProgram(root, tea.WithAltScreen())`. Root
`Init()` returns `tea.Batch(initial ledger load, initial checks run)`; `appMode` starts from
the load result.

### 6.8 Responsive layout (ticket 07)

- **Floor ≈ 100×30**, global. Below it the root renders only a full-screen
  "terminal too small — need ≥W×H, have w×h" message; only `q` is live. Exact numbers tuned
  in the shell build.
- **Degradation = priority drop.** Panels shrink proportionally to their minimums, then the
  lowest-priority panels are hidden — on both axes. Each view declares its own panel
  priority order. No whole-view scroll, no stacking.
- **Hidden-panel hint**: when a view drops panels, the status bar shows a dim
  `N panels hidden · widen terminal`. No zoom key.
- **`ui/layout` helper** — one pure 1-D primitive plus a frame:
  - `Allocate(total int, slots []Slot{Weight, Min, Priority}) []int` — 0 = dropped. Views
    nest it: one call for columns, one per column for panel heights.
  - `Panel(title, w, h, body)` — rounded border, title inset 2 cells.
  - Not a declarative layout tree; views own their structure.
- **Popups** (§6.5): full width; height = content, capped at ½ screen height; each popup
  reports its natural size and the root clamps it. Over the cap the checks list scrolls
  (arrows); the `?` popup reflows to `min(4, floor(w/minColW))` columns.
- **Deferred to the shell build** (implementation, not decisions): exact floor numbers,
  per-view priority orders, inter-panel gap cells, `Allocate` leftover-cell distribution
  and priority tie-break.

---

## 7. Configuration & theming

Tickets 10 and 15. **konto has no config file and no state file in v1** — every candidate
setting was either fixed or moved into the journal as an `account`-directive tag.

### 7.1 Journal path (`internal/config`)

Order: `--file` flag → `$LEDGER_FILE` → `~/.hledger.journal`.
Resolve to an **absolute path at startup**; pass `-f <path>` explicitly on every hledger
call. Nothing resolves to an existing file → first-run screen (§4.5).

`internal/config` only does this. Journal-intrinsic settings are `account` tags (§4.2);
the check set is fixed (§4.6); CSV files are chosen with the Bubbles `filepicker` opening in
the XDG download directory (no remembered directory). A config file is added only when a
real user setting appears.

### 7.2 ANSI-16 palette (`internal/ui/theme`, ADR 0005)

`lipgloss.Color("N")`, never hex — the TUI inherits the terminal theme. Fixed map, not
user-remappable (no override planned — the terminal theme is the colour knob):

| role | ANSI |
|---|---|
| background, foreground | *unset* (terminal default) |
| dim (labels, borders, secondary) | `8` |
| selection background | `8` (paired fg `15`) |
| red (negative, error) | `1` |
| green (positive) | `2` |
| amber (warning, focus, manual-source `✎`) | `3` |
| blue (mode indicator; forecast account line) | `4` |
| magenta (rule / price directive text) | `5` |
| cyan (panel titles) | `6` |
| bright-white (emphasis, grand totals) | `15` |

Rule: normal `1`–`7` for standard roles; bright `8`/`15` only for de-emphasis/emphasis.
`theme` exposes named `lipgloss.Style` values (`theme.Dim`, `theme.Negative`,
`theme.PanelTitle`, `theme.Selected`, `theme.Emphasis`, …); view code uses the names, never
raw indices.

---

## 8. Testing

Ticket `11-test-seams-and-fixtures`. Three seam levels matching the layers:

1. **Use cases** — test against a hand-written fake of the use case's own port interface, in
   that package's `_test.go`. No adapter, no hledger, no file. The bulk of logic tests.
2. **`internal/hledger`** — unit tests fake the unexported `run()` and assert (a) the arg
   vector and (b) the mapped `domain` value, against small committed output fragments under
   `internal/hledger/testdata/`. Plus `//go:build integration` tests that run the **real**
   `hledger` against fixture journals (catches JSON-schema drift), CI only, skipped locally.
3. **`internal/journal`** — pure text in/out. Table tests + golden output files under
   `internal/journal/testdata/`. No hledger needed.

4. **Architecture** — `internal/archtest` (test-only package) parses every Go file and
   enforces the design: the package → layer map and allowed layer dependencies (§2–§3, ADR
   0003/0004/0012; an unlisted package fails, so a new package is a deliberate edit there),
   Charm v2 as the only third-party code and only in `internal/ui` + `cmd`, no I/O in
   `domain`, `os/exec` only in `hledger` / `cmd` / ui root, no `time.Now` in `domain` or use
   cases (ADR 0012), `lipgloss.Color` only in `ui/theme` with a literal `"0"`–`"15"` and no
   hex literals (ADR 0005). Each rule has a self-test proving it catches its violation.

**Fixture journals** — hand-written, small, one concern each, committed under
`testdata/journals/` (no separate task; building them is part of the build):
`minimal`, `multicommodity` (EUR/PLN/CZK), `periodic` (`~` rules), `prices`,
`broken-assertion`, `accounts` (tagged + untagged), `import/` (`.csv` + `categories.rules`).

**e2e** — Gherkin features in `features/` run by `e2e/` (godog + pty + virtual terminal against
the compiled binary, real hledger behind a recording shim); `features/README.md`. Scenarios
carry `@wip` until their behaviour is built.

**Static analysis** — `.golangci.yml` enforces every Clean Code (Martin, 2nd ed.) rule a tool can
judge, each linter annotated with its rule; the `clean-review` skill reviews the rest plus DDD and
Pragmatic Programmer rules.

**TUI** — bulk = unit tests on each view's pure `Update` + `ui/widgets` render helpers
(string in / string out at fixed width). Smoke = a few full-program `teatest` flows (launch
→ nav each view → `!` → quit); build-time check that `teatest` has a Bubble Tea v2 release,
else hand-drive a `Program` loop. Golden `View()` snapshots **sparingly** — tab bar / status
bar at fixed sizes only, not whole views during active development.

---

## 9. Dev tooling

- **`mise`** provisions and pins the toolchain (`mise.toml`): `go`, `golangci-lint`,
  `gotestsum`, `gremlins` (mutation), `hledger`. `mise install`, then `mise tasks`.
- Tasks: `build`, `vet`, `lint`, `test`, `test:integration` (`-tags integration`, real
  `hledger`), `cover` / `cover:html` / `cover:check`, `mutation` / `mutation:diff`, and
  `check` (build + vet + lint + cover:check — the CI gate).
- **Quality gates start maxed** (user directive): line coverage `THRESHOLD` = 100;
  gremlins efficacy + mutant-coverage thresholds = 100. Loosening any threshold needs a
  recorded reason in `mise.toml`.
- `gofmt` / standard Go idioms; match surrounding code.
- CI: `.github/workflows/qa.yml` — jobs `check` (tidy + build/vet/lint/archtest/unit
  tests at 100% coverage), `vuln` (govulncheck), `integration`, `e2e`, `mutation-diff` on pull
  requests, full `mutation` nightly. Each job is one mise task; `mise run qa` runs the PR set
  locally.

---

## 10. Domain rules that must hold (from `CONTEXT.md` + the handoff)

- An account is **manual or import-fed, never both**.
- **Forecast transactions are never written** to the journal — it is a read-only
  `hledger balance --forecast`-style projection.
- **CSV dedup** derives the watermark from the journal (ADR 0009); konto surfaces skipped
  rows as a **list**, not a count.
- **No overlapping import rules**; a failing import balance assertion blocks the write.
- On CSV import, konto **writes a `= <balance>` assertion** on the account's posting of the
  newest imported transaction, stating the bank's closing balance for the export (Postbank:
  the `Kontostand` trailer) — this is the reconciliation mechanism.
- **Valuation**: not in v1 (EUR-only UI, ADR 0011). When it returns: "at cost" keeps
  multi-commodity totals as multiple amounts; `-V` collapses to one; UI copy explains the
  delta is unrealized FX gain, not spending.
- All layout is **responsive to `tea.WindowSizeMsg`**, recomputed from real terminal
  columns/rows — never hardcoded to the mock's 180×60.
- Colours are **ANSI indices, never hex**.
- **Periodic rules never post** — they are forecast inputs only (ADR 0006).
- **The forecast never guesses** — current balances + periodic rules only (ADR 0007).
- **Closed accounts** are visible only in the Dashboard Accounts panel; closing requires zero
  balance and no referencing `~` rule.
- UI copy says **"currency"**, never "commodity" (code/domain keep `Commodity`).
- UI copy says **"category"** for `expenses:*`/`income:*` and **"account"** for
  `assets:*`/`liabilities:*`; code and journal say "account" for both.
- **Every transaction is categorised** — or is a transfer (counterpart another account) or an
  opening balance; no catch-all account; import writes only once all
  rows have a category (no skip); entry (once it returns) requires one (ADR 0008).
- **Every hledger check passes** after every konto write (§4.6).

---

## 11. Deliberate UX deviations from the handoff

The handoff (`design_handoff_hledger_tui/README.md`) is otherwise authoritative.

- **ADR 0002** — the Dashboard **Accounts panel is a flat, non-indented list** (full account
  names + balance + source glyph). No account tree, no parent/header rows, no subtotals.
  Balance sheet / income statement hierarchies are unaffected.
- **ADR 0006** — the **Scheduled view has no Enter-to-post and no `p` pause**; periodic rules
  are forecast-only. `n`/`e` use an in-TUI form (not free text); cadence shows the raw
  period expression, not plain English.
- **ADR 0007** — the **Forecast view has no scenarios, no drivers panel, no projection
  table, no averages**; it shows one chart per currency (line per account) plus a milestone
  table.
- **ADR 0008** — **no `?` uncategorised marker and no `c` categorise key** on the Dashboard;
  transactions cannot be uncategorised.
- **ADR 0009** — **Import has no watched-folder file list, no `.latest`, no `d`/`u`
  override**; files come from a filepicker, dedup from the journal.
- **ADR 0010** — **no Entry view in v1**: five tabs on `1`–`5`, no manual-account UI, no
  classify-on-open prompt; accounts are created by first import.
- **ADR 0011** — **keyboard and chrome**: arrows not `j`/`k`; views on `1`–`5` only (no `:`
  command line, no `g` prefix / which-key goto, no mode badge); no panel focus; EUR-only UI
  (no `v`, no Market Prices panel); no Budget panel; no Dashboard Journal Checks panel or
  stat card (check status lives in the status bar); no Reports drill-in; no `y` export; no
  Recent `e` edit; `!` panel lists issues only (no `OK` rows); Reports period is `p`,
  `quarter` is quarter-to-date, and the income statement has no `% share` column.

- **Glossary** — the handoff's *Cash Flow* stat card is **`<MONTH> NET INCOME`**: it shows
  income − expenses, which hledger calls net income; hledger's *cashflow* (`cf`, change in
  cash accounts) is a different figure (CONTEXT.md).

New deviations get an ADR and a line here.

---

## 12. Deferred — grilled just-in-time during the build

Not in this spec by design. Each is a wayfinder ticket under `.scratch/konto-spec/issues/`,
grilled immediately before the `tdd-loop` pass for its view:

None open — 07 (responsive layout) resolved into §6.8, 08/09/13/15 into §4.8–§4.10 and §7.
Dashboard, Reports have no ticket; grill inline if a question surfaces. Entry is out of v1
(ADR 0010).

### 12.1 Large journals

Every hledger call parses the whole journal (about 1.1 s at 20 000 transactions; startup makes ~23 calls
side by side). konto does not cache or checkpoint. It owns a year-partitioned ledger and loads only this
year and last year (ADR 0013): `main.journal` holds directives and rules, transactions live in
`YYYY.journal` files, and carry transactions keep earlier balances right. Reports, dashboard spend and
checks stop at the window's start.

---

## 13. Suggested build order

Test-first (`/tdd-loop`), each slice the smallest nameable behaviour:

1. **Shell** — `cmd/konto` skeleton, `hledger` version preflight, empty root model, alt
   screen, quit. (ntcharts/v2 build check already passed — §1.)
2. **Chrome** — tab bar (five tabs, active styling), horizontal rule, status bar (view
   name + hint + `n/total` + check status + `? keys`), all responsive to `WindowSizeMsg`. `theme` package. `ui/layout` `Allocate` + `Panel` and
the too-small floor screen (§6.8).
3. **Navigation** — number keys `1`–`5` over empty view stubs.
4. **Overlays** — `lipgloss.Compositor` wrapper in `ui/layout`; `?` keys popup, `!` checks
   panel shell.
5. **Ledger load** — `internal/hledger` `run()` + `Client.Print`/`Balance`; `internal/ledger`
   `Loader`; first-run + parse-error `appMode`s; stale-while-revalidate; `r` refresh.
6. **Dashboard first panel** — the flat Accounts list from `Ledger.Accounts` (source glyphs,
   negative-in-red), `internal/journal` `account`-directive tag reading, the
   unclassified-account WARN (no prompt, ADR 0010).
7. Then per view — grill its ticket, build it.

---

*Foundation spec complete. Update it as deferred tickets resolve and as the build teaches
us things.*
