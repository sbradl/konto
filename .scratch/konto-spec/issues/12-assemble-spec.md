# 12 Assemble docs/spec.md

Type: task
Status: resolved
Blocked by: 02, 03, 04, 05, 06, 10, 11

## Question

Not a decision — the synthesis step. Writes the **foundation spec** (destination re-scoped
2026-09-10: foundation + incremental build, per-feature design deferred).

Once tickets 01–06, 10, 11 are closed, write `docs/spec.md` from the resolved tickets,
`CONTEXT.md`, and `docs/adr/`. It should:

- Reference `docs/design_handoff_hledger_tui/README.md` for all UX, not restate it.
- Cover the settled foundation: hledger integration (subcommands, output parsing, invocation
  architecture), domain model, package/module layout (ports-and-adapters), Bubble Tea model
  composition, overlay mechanics, config + theming, test seams + fixtures, and a short
  dev-tooling conventions section.
- Mark the per-feature areas as **grilled just-in-time during the build**: CSV import
  subsystem (08/14), forecast computation (09), periodic transaction subsystem (13),
  responsive layout details (07 — prototype during shell build).
- List the deliberate UX deviations from the handoff (ADR 0002: flat Accounts panel; plus
  any later ones) in one place.

Answer records the path to the finished spec.

## Answer

**`docs/spec.md`** — the foundation spec, 13 sections: stack + versions, ports-and-adapters
architecture (§2), package tree (§3), hledger integration (§4 — commands, the `internal/journal`
layer, invocation, loading/caching, errors/preflight, checks, `$EDITOR`), domain model (§5),
Bubble Tea composition (§6 — root model, routing, View interface, keymaps, overlays,
components, startup), config + theming (§7), testing (§8), dev tooling (§9), the hard domain
rules (§10), UX deviations from the handoff (§11), the deferred JIT tickets (§12), and a
suggested build order (§13).

Synthesised from tickets 01–06, 10, 11, `CONTEXT.md`, and `docs/adr/0001`–`0005`. References
the handoff for all UX. `CLAUDE.md` rewritten to point new sessions at spec + CONTEXT + ADRs
first.

Per-feature areas (07, 08/14, 09, 13, 15) are explicitly marked deferred — grilled just
before the `tdd-loop` pass for their view.
