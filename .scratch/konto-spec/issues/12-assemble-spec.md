# 12 Assemble docs/spec.md

Type: task
Status: open
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
