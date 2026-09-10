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
- **Environment fact** (charting machine, 2026-09-10): `hledger` is not installed and no
  journal file exists. The test-seams ticket must resolve how konto is tested without one;
  it may graduate an install/fixture task.

## Decisions so far

<!-- one line per closed ticket: gist of the answer, then the link for detail -->

_(none yet)_

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
- **`ntcharts` capability check** — confirm the braille line chart supports the dual-series
  actual+forecast join and thousands-axis labelling the handoff's Forecast/Dashboard charts
  need. Folded into the responsive-layout prototype for now.
- **Dev tooling conventions** — golangci-lint config, test runner, CI. Minor; likely a short
  section in spec assembly rather than its own ticket.

## Out of scope

<!-- work consciously ruled beyond the destination; never graduates -->

- **Writing any konto application code** — this map is planning-only (see Destination).
- **Revising the design handoff's UX** — views, navigation model, feature scope are frozen.
