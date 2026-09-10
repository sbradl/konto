# 04 Go package and module layout

Type: grilling
Status: open
Blocked by: 02, 03

## Question

Decide konto's package tree and the seams between parts:

- Top-level layout: `cmd/konto` entrypoint; `internal/...` for everything else?
- Where the hledger gateway lives (`internal/hledger`), where the domain types live
  (`internal/domain` or co-located), where UI lives.
- UI packaging: one `internal/ui` package for all six views, or one package per view? How
  shared UI primitives (layout helpers, theme, keymap, overlay compositor) are packaged so
  view packages don't import each other.
- The TUI ↔ domain seam: do view models call the gateway directly, or only a service layer?
  What's the dependency direction rule (UI depends on domain, never the reverse)?
- Where CSV-import logic sits — its own package (`internal/csvimport`) given it's the
  largest konto-owned subsystem?

Feeds **Bubble Tea model composition** and **Assemble docs/spec.md**.
