# 12 Assemble docs/spec.md

Type: task
Status: open
Blocked by: 02, 03, 04, 05, 06, 07, 08, 09, 10, 11, 13, 14

## Question

Not a decision — the synthesis step that produces the destination artifact.

Once every decision ticket is closed, write `docs/spec.md`: a build-ready implementation spec
for konto assembled from the resolved tickets, `CONTEXT.md`, and `docs/adr/`. It should:

- Reference `docs/design_handoff_hledger_tui/README.md` for all UX, not restate it.
- Cover: hledger integration (subcommands, output parsing, invocation architecture), domain
  model, package/module layout, Bubble Tea model composition, overlay + responsive layout
  mechanics, CSV import subsystem, periodic transaction subsystem, forecast computation,
  config + theming, test seams + fixtures, and a short dev-tooling conventions section.
- Include a per-view implementation-notes section (or point at per-view tickets if the fog
  graduated into those).
- List the deliberate UX deviations from the handoff (ADR 0002: flat Accounts panel; plus
  any later ones) in one place.
- Be executable by a fresh session or `/to-tickets` with nothing left to decide.

Answer records the path to the finished spec.
