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
- **Pre-decided**: the `hledger` binary is a hard runtime dependency. konto shells out to it;
  no Go reimplementation or journal-parser port is pursued. The open integration question is
  only *which subcommands / output format / invocation architecture*.
- **Skills to consult** per ticket type: `/grilling` + `/domain-modeling` for decision
  tickets (the default), `/research` subagent for research tickets, `/prototype` for
  prototype tickets. `craft-review` and `tdd-loop` are for the later build, not this map.
- **Environment fact** (dev machine, 2026-09-10): `hledger 1.52.1` is now installed
  (`/usr/bin/hledger`, installed during ticket 01 research). No journal file exists yet —
  the test-seams ticket still owns fixture-journal strategy, but "install hledger" is done.

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

## Not yet specified

<!-- in-scope fog, too dim to ticket precisely; graduates as the frontier advances -->

- **Per-view implementation notes** — once the domain model, model composition, and layout
  strategy are locked, each of the six views likely needs a short spec section (which
  Bubbles components, which hledger calls, which panels are overlay vs inline). May graduate
  into one ticket per view, or fold straight into spec assembly.
- **Journal writes beyond CSV import** — manual Entry-screen commits and Scheduled-screen
  "post next occurrence" both append journal text. Shape of that write path (which file,
  include-chain, check-satisfaction reporting) is dim until the hledger invocation
  architecture is settled.
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
