# konto — foundation spec

konto is a terminal UI frontend for [hledger](https://hledger.org), single-user personal
finance. This document is the **settled architectural foundation**: the decisions a build
session needs before writing code. It does not restate the UX — that lives in
[`design_handoff_hledger_tui/README.md`](design_handoff_hledger_tui/README.md), which is
authoritative and frozen (except the deviations in §11).

Per-feature design (CSV import, forecast, periodic transactions, responsive-layout details)
is **deliberately not in this spec** — it is grilled just-in-time as each view is built
(§12).

Sources: the wayfinder map and decision tickets under `.scratch/konto-spec/`, `CONTEXT.md`
(domain glossary), `docs/adr/0001`–`0005`.

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

**One build-time check before the stack is locked** (ticket 07): `ntcharts/v2`'s `go.mod`
carries `replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 ...` which
konto's build ignores. Build a trivial ntcharts chart against real `charm.land/bubbletea/v2`
first. A failure is the only thing that would force the Charm v1 stack (fallback:
`rmhubbert/bubbletea-overlay` for overlays).

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
internal/{ledger,report,csvimport,periodic,forecast,entry}
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
  domain/                types + pure rules (Amount arithmetic/format, MultiAmount helpers,
                         ReportRow.Kind, check-severity-by-name). No I/O.

  hledger/               ADAPTER. unexported run(ctx, args...) exec chokepoint + exported
                         Client (Balance, Print, Register, ARegister, Prices, Check,
                         ImportDryRun, Version). hledger JSON/text ⇄ domain. No tea import.
  journal/               ADAPTER. targeted line reader/writer of journal text (~ blocks,
                         account directives + tags, .latest files). NOT a parser.

  ledger/                USE CASE. Loader + the Ledger snapshot type; load/refresh by
                         composing hledger + journal.
  report/                USE CASE. Reports: native + market bs/is, diff for the FX footnote.
  csvimport/             USE CASE. import staging, .rules generation, .latest replay. (t.08)
  periodic/              USE CASE. ~ rule parse/edit/post-occurrence. (ticket 13)
  forecast/              USE CASE. read-only projection. (ticket 09)
  entry/                 USE CASE. draft-entry validation, commit, account classification.
  config/                config load + path resolution.

  ui/                    ui.go = root model, tab chrome, app-level keymap dispatch, wiring.
    theme/               ANSI-16 palette → named lipgloss.Style values.
    layout/              responsive layout helpers + the lipgloss.Compositor overlay wrapper.
    keys/                key.Binding sets (global + per view); the CMDS / HINTS / goto tables.
    widgets/             shared render helpers (stat card, panel frame, selectable-list
                         styling, ntcharts wrapper).
    dashboard/ imports/ entry/ scheduled/ forecast/ reports/
                         one package per view. Imports domain + the use-case service(s) it
                         needs + the ui leaves. NEVER imports hledger/journal. NEVER imports
                         another view.
```

Naming: the Import view package is `imports` (avoids shadowing `import`); its use case stays
`csvimport`. Two app/view name clashes (`forecast`, `entry`) — the view aliases its service
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
  indented postings + trailing comment/tags from journal text.
- **`account` directives and their tags** — the `konto:source:manual|import` tag
  (§7). `accounts --directives` drops tags; the tag *does* inherit onto that account's
  postings in `print -O json`, but text parsing is the primary source.
- **`.latest.<file>` watermark files** — read and write (hledger's dedup convention;
  ticket 08).
- **Writes**: append a new transaction, add/modify an `account`-directive tag (creating the
  directive if absent), append a posted occurrence, toggle a `~` rule's pause state, write
  `.rules` blocks.

It gets the include-set file list from `hledger files` and works per file. It never needs to
understand transactions, amounts, or include resolution.

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
  automatically on return from `$EDITOR`. **No filesystem watcher in v1.**
- **Granularity**: any journal write → **full `Ledger` refresh**. A `.rules` edit does not
  touch the journal — it invalidates import-staging state only.
- **In-flight UI contract**: stale-while-revalidate. The held snapshot always renders; a
  refresh sets an in-flight flag shown as a status-bar spinner; panels never blank. First
  load shows a centred "loading journal…". A failed refresh keeps the stale snapshot.

### 4.5 Errors and preflight

| Situation | Behaviour |
|---|---|
| `hledger` missing on `PATH`, or version `< 1.32` | **Refuse to start.** Preflight in `main` before the TUI: `exec.LookPath` → `hledger --version` → parse `hledger X.Y(.Z)` → compare. Print problem + minimum to stderr, exit non-zero. Version also stored in `LedgerMeta`. |
| No journal file configured / file absent | **First-run screen** offering to create a minimal skeleton journal at a chosen path. |
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
- Check set: basic (`parseable`, `autobalanced`, `assertions`) always; strict (`balanced`,
  `commodities`, `accounts`) default on; opt-in (`ordereddates`, `uniqueleafnames`, `tags`,
  `recentassertions`) default off. The on/off config is deferred (ticket 15).
- konto's own checks (unclassified account, and future ones) run in Go over the `Ledger` and
  **enumerate fully**.
- Severity assigned by check name (§7 `CheckIssue`).

### 4.7 `$EDITOR`

`tea.ExecProcess` (Bubble Tea v2) suspends the TUI, runs `$VISUAL` → `$EDITOR` → a sane
default with `+<line> <file>`, resumes, then fires a full refresh. Used by the `!` panel's
jump-to-location and the `e` edit key on list views.

---

## 5. Domain model

Glossary: `CONTEXT.md`. Full type sketch: ticket `03-domain-model-core-types`.

### 5.1 Value types (`internal/domain`)

- **`Amount`** — `{Commodity, Mantissa int64, Scale int32}`. Exact mirror of hledger's wire
  form. Hand-rolled arithmetic (add, negate, compare, rescale, format); guard aggregation
  against int64 overflow. No external decimal library.
- **`Commodity`** — `{Symbol string, Style}`; `Style` = hledger's `astyle` (side, spacing,
  decimal mark, digit groups, precision). `Amount.Format()` honours it.
- **`MultiAmount []Amount`** — one entry per commodity, hledger's order. `Single() (Amount,
  bool)`, `ExtraCount() int` (= `len-1`), `IsAllNegative() bool`. Market valuation → `len 1`.
- **`ValuationMode`** = `Native | Market`. A gateway query parameter (§4). No valuation math
  in konto.

### 5.2 Entities (mapped from hledger by the adapter)

- **`Transaction`** `{Index, Date, Status, Code, Description, Comment, Tags, Source
  SourcePos, Postings []Posting, Uncategorized bool}`. `SourcePos{File, Line}` for
  jump-to-`$EDITOR`. `Uncategorized` is computed.
- **`Posting`** `{Account, Amounts MultiAmount, BalanceAssertion *Assertion, Status,
  Comment, Tags, Type}` (`Type` = Regular/Virtual/BalancedVirtual).
- **`Assertion`** `{Amount, Inclusive bool, Total bool, Pos SourcePos}`.
- **`PriceDirective`** `{Date, From Commodity, Price Amount}` (from `hledger prices` text).
- **`PeriodicRule`** — named only; fields, pause mechanism, write path are ticket 13.
- **`CheckIssue`** `{Check string, Severity ERR|WARN, Message, File string, Line, Col int}`.
  `CheckRun` aggregates issues + clean/dirty for the `● clean` indicator. Severity by check
  name: `parseable`/`autobalanced`/`assertions`/`balanced` = ERR; `commodities`/`accounts`/
  `ordereddates`/`uniqueleafnames`/`tags`/`recentassertions` + konto's unclassified-account
  check = WARN.
- **`ReportTable`** `{Rows []ReportRow}` where `ReportRow{Kind, Label, Depth, Amounts
  []MultiAmount, Delta *MultiAmount, Share *float64}`, `Kind ∈ {SectionHeader, Leaf,
  NegativeLeaf, Subtotal, GrandTotal}` computed from structure + sign.

### 5.3 Account (ADR 0002)

- **`[]Account` — flat list of postable accounts only.** No parent/aggregation entities.
- **`Account.Source`** — optional `Manual | Import`, unset = unclassified. From the
  `konto:source:` tag on the `account` directive (§4.2).
- **Unclassified** (postable, no tag): excluded from Entry and import targets; a konto WARN;
  konto prompts manual/import on first open and writes the tag.
- Balance-report hierarchies are `ReportTable`, separate from `[]Account`.

### 5.4 konto-only entities (no hledger counterpart)

`ImportSource`, `CsvRow`, `ImportRule`, `RuleMatchStats`, `Watermark` — ticket 08.
`ForecastDriver`, `Scenario` — ticket 09.

### 5.5 `Ledger` — the loaded snapshot (`internal/ledger`)

No monolithic `Journal` type. `Ledger` `{Meta LedgerMeta, Accounts []Account, Transactions
[]Transaction, Prices []PriceDirective, Periodics []PeriodicRule, Checks CheckRun}`; each
field populated by its own gateway call, independently refreshable. `LedgerMeta`
`{JournalPath, TxnCount, HledgerVersion, LoadedAt, Health}` — backs the tab-bar chrome.
Views hold a read-only `*Ledger`, replaced wholesale on refresh, never mutated.

---

## 6. Bubble Tea composition (ADR 0004)

Full detail: ticket `05-bubble-tea-model-composition`.

### 6.1 Root model (`internal/ui`)

The program's single `tea.Model`. Holds:

- `appMode ∈ {firstRun, parseError, normal}` — first two render their own screen, not the
  tab UI, not a seventh view.
- the **six view sub-models**, constructed at startup, alive for the whole run. Tab switch
  changes which one renders + receives input; cursor/scroll/draft state survives.
- **app-global state**: `valuationMode`, `period`, `scenario`, terminal size, the read-only
  `*domain.Ledger`.
- **chrome**: active view id, tab bar, status bar. **overlay state**: `whichKeyMode`
  (`none|root|goto`), command-line (`active`, `text`, filtered `CMDS`), `checksPanelOpen`.
- the use-case **ports** it needs (`ledgerService`, checks) — injected by `cmd/konto`.

Views never reference each other. Cross-view effects are mediated: a view acts → state
changes → root broadcasts → every view updates.

### 6.2 View sub-model interface

```go
type View interface {
    Update(tea.Msg) (View, tea.Cmd)
    View() string          // renders into its content area (size via WindowSizeMsg)
    Title() string
    StatusHint() string    // handoff HINTS map
    KeyMap() []key.Binding  // feeds ? / which-key
}
```

Each `Update` is a pure `(Msg) → (View, Cmd)` — unit-testable with no running program. A
view holds the use-case port(s) it needs + its own view-local state (cursor, form field,
scroll, generation counter).

### 6.3 Message routing (root's `Update`)

| Message class | Routed to |
|---|---|
| `tea.KeyMsg`, `tea.MouseMsg` | open overlay if any, else the **active view only**. Global keys (`1`–`6`, `g`, `:`, `?`, `!`, `v`, `q`/`ctrl+c`) caught by root **first**; if the active view has a focused text input, only `Esc`/`Ctrl+C` are caught. |
| `tea.WindowSizeMsg` | root computes the content rect (window − tab bar − rule − status bar), forwards a **content-dimension** `WindowSizeMsg` to **all six** views |
| `ledgerLoadedMsg`, `valuationChangedMsg`, `checksLoadedMsg`, ticks | root updates its state, **broadcasts** to all six views |
| view-specific async results (`reportLoadedMsg`, …) | root forwards to the **one owning view** |

Shared-state changes are **messages, not shared pointers**. Async races: each request
carries a per-view monotonic generation number; stale results are dropped (pairs with §4.3
context cancellation). Sub-model `Cmd`s bubble up and are `tea.Batch`-ed by root.

### 6.4 Keymaps (`internal/ui/keys`)

`key.Binding` sets — one global + one per view. Root resolves against the global set first
(after overlay / text-input checks), else forwards to the active view. `?` and which-key
render from the binding sets (`bubbles/help`). `g` goto-prefix is a root state machine.

### 6.5 Overlays

Root-owned. The `:` command-line **input** replaces the status-bar row (handoff). The
filtered-command **dropdown**, the which-key popup, and the `!` checks panel are
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
| custom selectable-list widget (`ui/widgets`) | Dashboard panels (Recent, Upcoming, Accounts) |
| `bubbles/textinput` | Entry form fields, command line, rule-editor target |
| `bubbles/viewport` | scrollable panels (journal preview, checks body) |
| `bubbles/key`, `bubbles/help`, `bubbles/spinner` | bindings, `?` render, refresh indicator |
| `ntcharts` | Dashboard trend + Forecast braille charts |
| `bubbles/list` | evaluated, **not used** |

Final fit confirmed by the layout prototype (ticket 07 / shell build).

### 6.7 Startup

`cmd/konto`: config → hledger version preflight → construct adapters/services → construct
root model (inject ports + config) → `tea.NewProgram(root, tea.WithAltScreen())`. Root
`Init()` returns `tea.Batch(initial ledger load, initial checks run)`; `appMode` starts from
the load result.

---

## 7. Configuration & theming

Ticket `10-config-and-theming` (partial — build-critical only). Remaining config: ticket 15.

### 7.1 Journal path (`internal/config`)

Order: `--file` flag → `$LEDGER_FILE` → (future config `journal` key) → `~/.hledger.journal`.
Resolve to an **absolute path at startup**; pass `-f <path>` explicitly on every hledger
call. Nothing resolves to an existing file → first-run screen (§4.5).

For v1, `internal/config` only does this. A config file is introduced when the first real
setting needs one (ticket 15: file format/location, check on/off set, catch-all account
name for `Uncategorized`, import source dirs).

### 7.2 ANSI-16 palette (`internal/ui/theme`, ADR 0005)

`lipgloss.Color("N")`, never hex — the TUI inherits the terminal theme. Fixed map, not
user-remappable in v1:

| role | ANSI |
|---|---|
| background, foreground | *unset* (terminal default) |
| dim (labels, borders, secondary) | `8` |
| selection background | `8` (paired fg `15`) |
| red (negative, error) | `1` |
| green (positive) | `2` |
| amber (warning, focus, manual-source `✎`) | `3` |
| blue (mode indicator, forecast line) | `4` |
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

**Fixture journals** — hand-written, small, one concern each, committed under
`testdata/journals/` (no separate task; building them is part of the build):
`minimal`, `multicommodity` (EUR/PLN/CZK), `periodic` (`~` rules), `prices`,
`broken-assertion`, `accounts` (tagged + untagged), `import/` (`.csv` + `.rules` + `.latest`).

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
- CI (not yet wired — no remote): run `mise run check` plus `mise run test:integration`.
  Mutation testing runs out of the gate loop (`mutation:diff` on PRs, full `mutation`
  nightly) — it recompiles per mutant and is slow by nature.

---

## 10. Domain rules that must hold (from `CONTEXT.md` + the handoff)

- An account is **manual or import-fed, never both**.
- **Forecast transactions are never written** to the journal — it is a read-only
  `hledger balance --forecast`-style projection.
- **CSV dedup** uses hledger's `.latest.<file>` convention; konto surfaces skipped rows as a
  **list**, not a count, with a manual override.
- On CSV import, konto **writes a `= <balance>` assertion** on the last posting from the
  bank export's running-balance column — this is the reconciliation mechanism.
- **Valuation**: "at cost" keeps multi-commodity totals as multiple amounts; `-V` collapses
  to one; UI copy explains the delta is unrealized FX gain, not spending.
- All layout is **responsive to `tea.WindowSizeMsg`**, recomputed from real terminal
  columns/rows — never hardcoded to the mock's 180×60.
- Colours are **ANSI indices, never hex**.

---

## 11. Deliberate UX deviations from the handoff

The handoff (`design_handoff_hledger_tui/README.md`) is otherwise authoritative.

- **ADR 0002** — the Dashboard **Accounts panel is a flat, non-indented list** (full account
  names + balance + source glyph). No account tree, no parent/header rows, no subtotals.
  Balance sheet / income statement hierarchies are unaffected.

New deviations get an ADR and a line here.

---

## 12. Deferred — grilled just-in-time during the build

Not in this spec by design. Each is a wayfinder ticket under `.scratch/konto-spec/issues/`,
grilled immediately before the `tdd-loop` pass for its view:

| Area | Ticket | Prerequisite |
|---|---|---|
| Responsive layout helper shape | 07 | done *in* the shell build; also runs the ntcharts/v2 check |
| CSV import subsystem (Postbank + format-extensible) | 08 | **14** — user provides real Postbank CSV exports into `.scratch/konto-spec/samples/` |
| Forecast computation (trailing averages, scenarios, drivers) | 09 | — |
| Periodic transaction subsystem (`~` parse/edit, pause, post-occurrence) | 13 | — |
| Config file format + remaining settings | 15 | — |

---

## 13. Suggested build order

Test-first (`/tdd-loop`), each slice the smallest nameable behaviour:

1. **Shell** — `cmd/konto` skeleton, `hledger` version preflight, empty root model, alt
   screen, quit. Confirms the stack (run the ntcharts/v2 build check here).
2. **Chrome** — tab bar (six tabs, active styling), horizontal rule, status bar (mode + view
   name + hint + `? keys`), all responsive to `WindowSizeMsg`. `theme` package.
3. **Navigation** — number keys `1`–`6`, the `g` goto-prefix state machine, `:` command line
   (typeahead over `CMDS`, Tab-complete, Enter navigates), over empty view stubs.
4. **Overlays** — `lipgloss.Compositor` wrapper in `ui/layout`; which-key popup (`g` / `?`),
   `!` checks panel shell.
5. **Ledger load** — `internal/hledger` `run()` + `Client.Print`/`Balance`; `internal/ledger`
   `Loader`; first-run + parse-error `appMode`s; stale-while-revalidate; `r` refresh.
6. **Dashboard first panel** — the flat Accounts list from `Ledger.Accounts` (source glyphs,
   negative-in-red), `internal/journal` `account`-directive tag reading, the
   unclassified-account prompt + WARN.
7. Then per view — grill its ticket, build it.

---

*Foundation spec complete. Update it as deferred tickets resolve and as the build teaches
us things.*
