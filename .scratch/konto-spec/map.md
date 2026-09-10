<!-- wayfinder:map -->
# Map: konto build-ready spec

## Destination

`docs/spec.md` — a build-ready implementation spec for konto (the hledger TUI), written so a
fresh implementation session (or `/to-tickets`) can execute it without re-deciding anything.
Companion artifacts: `CONTEXT.md` glossary and `docs/adr/` entries for hard-to-reverse calls.
**No application code is written during wayfinding** — this map produces decisions, not a build.

Done when every decision ticket is closed and `docs/spec.md` is assembled from them.

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
  satisfy them structurally, `cmd/konto` wires.
- **Domain glossary** is `CONTEXT.md`; **ADRs** live in `docs/adr/`. Both now exist.
- **Pre-decided**: the `hledger` binary is a hard runtime dependency. konto shells out to it;
  no Go reimplementation or journal-parser port is pursued. The open integration question is
  only *which subcommands / output format / invocation architecture*.
- **Skills to consult** per ticket type: `/grilling` + `/domain-modeling` for decision
  tickets (the default), `/research` subagent for research tickets, `/prototype` for
  prototype tickets. `craft-review` and `tdd-loop` are for the later build, not this map.
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
  `konto:source:` tag on the `account` directive — unclassified accounts prompt on open and
  WARN. `ValuationMode` is a gateway query param (re-fetch with `-V`). `Ledger` is a
  snapshot of independently-refreshable slices, not a monolithic type. Glossary in
  `CONTEXT.md`. Graduated the *Periodic transaction subsystem design* ticket.

## Not yet specified

<!-- in-scope fog, too dim to ticket precisely; graduates as the frontier advances -->

- **Per-view implementation notes** — once the domain model, model composition, and layout
  strategy are locked, each of the six views likely needs a short spec section (which
  Bubbles components, which hledger calls, which panels are overlay vs inline). May graduate
  into one ticket per view, or fold straight into spec assembly.
- **Journal write targets** — write *mechanism* is settled (`internal/journal` owns
  line-oriented mutation; full `Ledger` refresh after each write). Still dim: *which* file
  in an include chain each construct is appended to, and the Entry-view commit specifics
  (which file, include-chain, check-satisfaction reporting). Post-occurrence writes are
  owned by *Periodic transaction subsystem design*; the Entry commit path graduates once a
  view is designed for it.
- **Keymap / command-line dispatch detail** — the `CMDS` / `HINTS` / goto-prefix tables from
  the mock's inline JS need transcribing into a Go keymap model; precise structure waits on
  the Bubble Tea model composition decision.
- **`ntcharts` capability + v2 build check** — folded into *Responsive layout strategy*:
  confirm the braille line chart supports the dual-series actual+forecast join and
  thousands-axis labelling, AND that `ntcharts/v2` compiles against the real
  `charm.land/bubbletea/v2` (its `go.mod` `replace` is ignored by konto's build). A build
  failure here is the one thing that could push konto back to the Charm v1 stack.
- **Dev tooling conventions** — golangci-lint config, test runner, CI. Minor; likely a short
  section in spec assembly rather than its own ticket.

## Out of scope

<!-- work consciously ruled beyond the destination; never graduates -->

- **Writing any konto application code** — this map is planning-only (see Destination).
- **Revising the design handoff's UX** — views, navigation model, feature scope are frozen.
