# 04 Go package and module layout

Type: grilling
Status: resolved
Blocked by: 02, 03

## Question

Decide konto's package tree and the seams between parts:

- Top-level layout: `cmd/konto` entrypoint; `internal/...` for everything else?
- Ticket 02 introduced working names to place/confirm: `internal/hledger` (gateway),
  `internal/journal` (targeted line-oriented reader/writer), and a `Loader` that composes
  both into the `Ledger` snapshot — where does `Loader` live (domain? its own `ledger`
  package?).
- Where the domain types live (`internal/domain` or co-located), where UI lives.
- UI packaging: one `internal/ui` package for all six views, or one package per view? How
  shared UI primitives (layout helpers, theme, keymap, overlay compositor) are packaged so
  view packages don't import each other.
- The TUI ↔ domain seam: do view models call the gateway directly, or only a service layer?
  What's the dependency direction rule (UI depends on domain, never the reverse)?
- Where CSV-import logic sits — its own package (`internal/csvimport`) given it's the
  largest konto-owned subsystem?

Feeds **Bubble Tea model composition** and **Assemble docs/spec.md**.

## Answer

Module: `github.com/sbradl/konto` (unchanged). **Ports-and-adapters (Clean Architecture)**,
with the Dependency Rule enforced by Go's structural interfaces — see
`docs/adr/0003-ports-and-adapters.md`. Every source dependency points inward toward
`domain`; the one boundary that would otherwise invert (use case → adapter) is bridged by a
**consumer-side port**.

### Circles, outer → inner

```
cmd/konto           Composition Root. The only package that imports concretes from every
                    circle. Loads config, runs the hledger version preflight, constructs the
                    adapters, injects them into the application services (as ports),
                    injects services into the root ui model, runs tea.Program.

internal/ui/…        Frameworks & Drivers + Interface Adapters (controller/presenter folded
                    in). Bubble Tea / Lip Gloss confined here. Views translate domain values
                    to rendered cells and key events to service calls wrapped in tea.Cmd.
  ui/               ui.go = root model, tab chrome, app-level keymap dispatch, wiring.
  ui/theme          ANSI-16 palette + lipgloss styles.
  ui/layout         responsive layout helpers + the lipgloss.Compositor overlay wrapper.
  ui/keys           keymap (bubbles/key), the CMDS / HINTS / goto tables.
  ui/widgets        shared render helpers (stat card, panel frame, list styling, ntcharts
                    wrapper).
  ui/dashboard  ui/imports  ui/entry  ui/scheduled  ui/forecast  ui/reports
                    one package per view. Imports domain + the application service(s) it
                    uses + the ui leaves. NEVER imports hledger/journal. NEVER imports
                    another view.

internal/{ledger,report,csvimport,periodic,forecast,entry}
                    Use Cases (application business rules). Plain Go — no tea, no lipgloss,
                    no exec. Each declares the PORT interfaces it needs from the outside
                    (e.g. report.ledgerGateway) and holds them as struct fields. Imports
                    domain only. Split by concern — no single app.App facade.

internal/hledger    Interface Adapter → the hledger binary. unexported run(ctx, args...)
                    exec chokepoint + exported Client (Balance, Print, Register, ARegister,
                    Prices, Check, ImportDryRun, Version). Maps hledger JSON/text to domain
                    values. Imports domain only. Satisfies the use-case ports STRUCTURALLY
                    (no import of the application packages).
internal/journal    Interface Adapter → the journal text file. Targeted line reader/writer
                    (~ blocks, account directives + tags, .latest files). Not a parser.
                    Imports domain only. Structurally satisfies its ports.

internal/config     Config load + path resolution. Imported by cmd only.

internal/domain     Entities. Types + pure rules (Amount arithmetic/format, MultiAmount
                    helpers, ReportRow.Kind, check-severity-by-name). No I/O. Imports
                    nothing of konto's. Everyone may import it.
```

### The port pattern (idiomatic Go DIP)

A use case names the interface it needs, unexported, in its own package:

```go
// internal/report
type ledgerGateway interface {
    BalanceSheet(p domain.Period, v domain.ValuationMode) (domain.ReportTable, error)
    IncomeStatement(p domain.Period, v domain.ValuationMode) (domain.ReportTable, error)
}
type Service struct { gw ledgerGateway }
func New(gw ledgerGateway) *Service { return &Service{gw} }
```

`*hledger.Client` already has methods with those signatures, so it satisfies `ledgerGateway`
with **no `implements` keyword and no import in either direction** — `report` imports
`domain`, `hledger` imports `domain`, neither imports the other. `cmd/konto` is where the
concrete meets the port:

```go
hl := hledger.New(cfg.HledgerBin)
reports := report.New(hl)          // *hledger.Client used as report.ledgerGateway
```

Consequences:
- **Dependency Rule holds everywhere.** No inner circle imports an outer one.
- **Adapters are the only mockable seam and they are fully swappable** — `report` etc. test
  against a hand-written fake `ledgerGateway`, never touching `internal/hledger` (ticket 11).
- Each use case declares its **own narrow** port even if two of them overlap (interface
  segregation); one `*hledger.Client` satisfies all of them.

### Deliberate deviations from strict Clean Architecture

- **Adapters return `domain` entities directly**, not adapter-specific response models
  (ADR 0001). `domain` *is* the inner circle's data form, so this respects the Dependency
  Rule; per-use-case request/response structs would be ceremony without payoff here.
- **No standalone Presenter.** View packages format `domain` → cells themselves. Acceptable
  for a single TUI with no alternate delivery mechanism.
- `domain` is **thin** — konto is a frontend, the accounting rules live in hledger.

### Naming

- View packages live at `internal/ui/<name>` with plain package names (`dashboard`,
  `imports`, `entry`, `scheduled`, `forecast`, `reports`). `imports` avoids shadowing the
  `import` keyword in tooling/prose; the app service stays `csvimport`.
- Two app/view package-name clashes (`forecast`, `entry`): the view importing its app
  service aliases it — `forecastsvc "…/internal/forecast"`.

### Downstream

- **Bubble Tea model composition** (now unblocked): root model in `ui`, sub-models in
  `ui/<view>`; how shared state and the `tea.Cmd` results route.
- Ticket 08 → `internal/csvimport`; ticket 09 → `internal/forecast`; ticket 13 →
  `internal/periodic`; ticket 10 → `internal/config`.
