<!-- wayfinder:map -->
# Map: konto build-ready spec

## Destination

**Foundation laid, build under way.** `docs/spec.md` captures the settled architecture
(tickets 01–06, 10, 11) + `CONTEXT.md` + `docs/adr/`; the app shell and first Dashboard
slices are built test-first on top of it. Per-feature design — CSV import (08/14), forecast
(09), periodic transactions (13) — is grilled **just-in-time** as each view is built.

Done when the foundation spec exists and every view is built; per-feature tickets close as
their views land.

## Notes

- **Domain**: terminal UI frontend for [hledger](https://hledger.org) (plain-text accounting),
  single-user personal finance. Target stack: Go + Bubble Tea + Lip Gloss + Bubbles + `ntcharts`.
- **Stack version decided** (via *Overlay and popup compositing mechanics*): the **Charm v2
  stack** — `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`
  (vanity import paths, *not* `github.com/charmbracelet/.../v2`), `ntcharts/v2`. v1 is frozen.
  Every downstream ticket (package layout, model composition, layout strategy) assumes v2.
- **UX is frozen** in `docs/design_handoff_hledger_tui/README.md` — authoritative, not
  re-litigated. Wayfinding decides *implementation* only (architecture, data model, package
  layout, test seams, overlay/layout mechanics, subsystem logic). The handoff README stays
  unedited; the spec references it for "what it looks like".
- **Deliberate UX deviations from the handoff** (each recorded as an ADR, all collected in
  the spec by ticket *Assemble docs/spec.md*):
  - `docs/adr/0002` — Dashboard Accounts panel is a flat, non-indented list (no account
    tree, no parent/subtotal rows).
- **Architecture**: `docs/adr/0003` — ports-and-adapters / Clean Architecture; every source
  dependency points inward to `internal/domain`; use cases declare their own ports, adapters
  satisfy them structurally, `cmd/konto` wires. `docs/adr/0004` — one root Bubble Tea model
  mediates all six views; app-global state in the root; propagation by message, not shared
  pointers.
- **Domain glossary** is `CONTEXT.md`; **ADRs** live in `docs/adr/`. Both now exist.
- **Pre-decided**: the `hledger` binary is a hard runtime dependency. konto shells out to it;
  no Go reimplementation or journal-parser port is pursued. The open integration question is
  only *which subcommands / output format / invocation architecture*.
- **Skills to consult** per ticket type: `/grilling` + `/domain-modeling` for decision
  tickets (the default), `/research` subagent for research tickets, `/prototype` for
  prototype tickets. `tdd-loop` + `craft-review` drive the build.
- **This effort carries execution** (overriding wayfinder's plan-only default, re-scoped
  2026-09-10): the foundation is decided; remaining work is the `tdd-loop` build plus JIT
  grilling of per-feature tickets (08 import, 09 forecast, 13 periodic) when their view is
  built. 14 (bank CSV samples, user-supplied) is a prerequisite for 08. 07 (responsive
  layout) was grilled 2026-09-30 ahead of the shell build; the shell build tunes numbers.
- **Environment fact** (dev machine, 2026-09-10): `hledger 1.52.1` is now installed
  (`/usr/bin/hledger`, installed during ticket 01 research). No journal file exists yet —
  the test-seams ticket still owns fixture-journal strategy, but "install hledger" is done.
- **CSV import scope**: primary bank is **Postbank** (German — expect `;` delimiter,
  Latin-1/CP1252 encoding, comma decimals, `DD.MM.YYYY`, possible preamble). The import
  subsystem must be **format-extensible** to other banks. *CSV import subsystem design*
  owns the abstraction; it is blocked on *Obtain sample bank CSV files* (the user supplies
  real Postbank exports — samples land in `.scratch/konto-spec/samples/`).

## Decisions so far

<!-- one line per closed ticket: gist of the answer, then the link for detail -->

- *Journal write targets + v1 scope* (grilled inline 2026-09-30, no ticket) — **no Entry
  view / manual-account UI in v1** (ADR 0010): five tabs `1`–`5`; model keeps
  `Manual | Import`. konto never runs on a preexisting ledger → no classify prompt;
  accounts are created by **first import** (unknown IBAN → create, directive in main
  journal; no attach-to-existing). Future Entry commit target = file of the account's
  latest transaction, else main journal, date-ordered. Spec §4.9, §5.3, §11.

- [Responsive layout strategy](issues/07-responsive-layout-strategy.md) — floor ≈100×30
  (too-small screen below); **priority drop** (shrink to mins, then hide low-priority
  panels, per-view order; dim `N panels hidden` note); `ui/layout` = pure 1-D `Allocate`
  nested by views + `Panel`; popups full width, height ≤ ½ screen, scroll/reflow. ntcharts/v2
  builds against real bubbletea v2 — stack locked. Spec §6.8.

- [Assemble docs/spec.md](issues/12-assemble-spec.md) — **`docs/spec.md`** written: the
  foundation spec synthesising tickets 01–06, 10, 11 + `CONTEXT.md` + ADRs 0001–0005, with
  a build order. `CLAUDE.md` rewritten to point at it. Per-feature areas (07, 08/14, 09,
  13, 15) marked deferred.
- [hledger machine-readable output](issues/01-hledger-machine-readable-output.md) —
  `-O json` for `print`/`register`/`aregister`/`bal`/`bs`/`is`/`cf`; **no** machine format
  for `check`/`accounts`/`commodities`/`prices`. `print` drops `~` rules (konto parses the
  journal itself for verbatim rule text); `check --strict` = first-failure-only prose on
  stderr, exit 1, no severities (run per-check + regex + fix-rerun to enumerate); forecast
  rows only separable via the `_generated-transaction` tag on `print`/`aregister --forecast`;
  `import --dry-run` doesn't list skipped rows (konto builds its own). **Version floor:
  `hledger >= 1.32`**, checked at startup, JSON decoded tolerantly. Full detail:
  `research/01-hledger-output.md`.
- [Overlay and popup compositing mechanics](issues/06-overlay-popup-compositing.md) —
  build on the **Charm v2 stack** and composite overlays with `lipgloss.Compositor` (v2's
  cell-grid `Layer`/`Compositor`); no community overlay package, no hand-rolled splicing.
  Cursor via `tea.View.Cursor`. One caveat handed to *Responsive layout strategy*: verify
  `ntcharts/v2` builds against real `charm.land/bubbletea/v2` (its `go.mod` carries an
  ignored `replace`). Full findings: `research/06-overlay-compositing.md`.
- [hledger invocation architecture](issues/02-hledger-invocation-architecture.md) — two
  data paths behind a `Loader` that builds the `Ledger` snapshot: an `internal/hledger`
  **gateway** (plain blocking Go, no `bubbletea` import; unexported `run()` exec chokepoint
  + typed `Client`; UI wraps calls in `tea.Cmd`) and an `internal/journal` **targeted
  line-oriented reader/writer** for `~` blocks, `account` tags, `.latest` files (not a
  parser). **Eager** load at startup, held snapshot, explicit `r` refresh + auto-refresh
  after writes and after `$EDITOR`; no watcher; any write → full refresh;
  stale-while-revalidate rendering. Startup preflight refuses to run without `hledger >=
  1.32`; missing journal → first-run skeleton-create screen; parse error → top-severity
  check issue. `!` panel runs each hledger check as its own invocation (first-failure only,
  fix→refresh→next), konto's own checks enumerate fully. `$EDITOR` via `tea.ExecProcess`.
- [Configuration and ANSI-16 theming](issues/10-config-and-theming.md) — *(partial —
  build-critical bits only)* journal path resolved `--file` → `$LEDGER_FILE` → config →
  `~/.hledger.journal`, to an absolute path, `-f` passed explicitly always. Fixed ANSI-16
  palette (ADR 0005) in `internal/ui/theme` as named `lipgloss.Style`s: dim/selection `8`,
  red `1` green `2` amber `3` blue `4` magenta `5` cyan `6`, emphasis `15`, bg/fg unset.
  Config-file format + check-set + catch-all account deferred → ticket 15.
- [Test seams and fixture strategy](issues/11-test-seams-and-fixtures.md) — three seam
  levels: use cases fake their own port; `internal/hledger` unit-tested with a faked
  `run()` + committed `testdata` output fragments, plus `//go:build integration` tests
  against real hledger in CI; `internal/journal` is pure text table/golden tests. Small
  one-concern fixture journals committed under `testdata/journals/` (no task ticket). TUI:
  bulk = unit tests on pure `Update`, smoke = `teatest`, golden snapshots only for tab/
  status bar. `go test ./...` + `-tags integration`.
- [Bubble Tea model composition](issues/05-bubble-tea-model-composition.md) — one root model
  (`internal/ui`) mediates six always-alive view sub-models (ADR 0004). Root owns
  app-global state (`valuationMode`, `period`, `scenario`, size, read-only `*domain.Ledger`),
  chrome, overlays, and the service ports. Routing: key/mouse → active view only (root
  catches global keys first); data msgs → broadcast to all; view-specific async results →
  routed to the owner with a per-view generation counter for stale-drop. Shared-state
  changes are messages. `View` interface = `Update/View/Title/StatusHint/KeyMap`. Keymaps in
  `ui/keys` (global set + per-view). Components: `bubbles/table` for grids, custom list
  widget for Dashboard panels, `textinput`/`viewport`/`help`/`spinner`, `ntcharts`;
  `bubbles/list` not used. First-run / parse-error are root `appMode`s, not views.
- [Go package and module layout](issues/04-package-module-layout.md) — **ports-and-adapters
  / Clean Architecture** (ADR 0003). Circles inward: `cmd/konto` (composition root) →
  `internal/ui` + one package per view (Bubble Tea confined here; import domain + the app
  services they use; never adapters, never each other; leaves `ui/theme|layout|keys|widgets`)
  → use cases `internal/{ledger,report,csvimport,periodic,forecast,entry}` (plain Go, each
  **declares its own port interfaces**, imports domain only, no `app.App` facade) →
  adapters `internal/hledger` + `internal/journal` (import domain only, satisfy the ports
  **structurally**) → `internal/domain` (entities + pure rules, imports nothing). `cmd`
  wires concretes into ports. Deviations from strict CA: adapters return domain entities
  (ADR 0001), no standalone presenter, thin domain.
- [konto domain model and core types](issues/03-domain-model-core-types.md) — **rich
  konto-owned model behind a gateway mapping layer** (ADR 0001); views never see hledger
  JSON. `Amount` = hand-rolled fixed-point `{Commodity, Mantissa int64, Scale int32}`, no
  deps; `MultiAmount []Amount` carries the "+ N cur" case. `[]Account` is a **flat list of
  postable accounts** (ADR 0002), `Account.Source` optional `Manual | Import` read from a
  `konto:source:` tag on the `account` directive — unclassified accounts WARN (prompt
  dropped by ADR 0010). `ValuationMode` is a gateway query param (re-fetch with `-V`). `Ledger` is a
  snapshot of independently-refreshable slices, not a monolithic type. Glossary in
  `CONTEXT.md`. Graduated the *Periodic transaction subsystem design* ticket.

## Not yet specified

<!-- in-scope fog, too dim to ticket precisely; graduates as the frontier advances -->

- **Per-view build + JIT design** — each of the five v1 views is built in its own `tdd-loop`
  pass; the views with real subsystem logic carry a grilling ticket done just before that
  pass: Import (08), Scheduled (13), Forecast (09) and config (15) all resolved 2026-09-30. Dashboard, Reports have
  no open design ticket — grill inline if a question surfaces during the build.
- **`CMDS` / `HINTS` tables** — transcribe the mock's inline-JS command list and per-view
  hint strings into the `internal/ui/keys` structures (shape decided in ticket 05); a
  mechanical build task, not a decision.

## Out of scope

<!-- work consciously ruled beyond the destination; never graduates -->

- **Entry view / manual-account UI in v1** (ADR 0010) — the model keeps manual accounts;
  the view returns after v1 over the unchanged model.
- **Revising the design handoff's UX** — views, navigation model, feature scope are frozen
  (except the recorded deviations under Notes).

<!-- "no application code" was removed 2026-09-10: the destination now carries the build. -->

