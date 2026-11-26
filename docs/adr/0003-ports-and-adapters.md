# Ports-and-adapters package architecture

konto follows Clean Architecture (Martin): every source dependency points inward toward
`internal/domain` (Entities). The circles, outer to inner:

- **`cmd/konto`** — Composition Root; the only package importing concretes from every circle.
- **`internal/ui/…`** — Frameworks & Drivers; Bubble Tea / Lip Gloss confined here. One
  package per view; views import `domain` + the application services they use, never the
  adapters, never each other.
- **`internal/{ledger,accounts,checks,carry,csvimport,periodic,entry}`** — Use Cases; plain Go
  (no `tea`, no `exec`). Each **declares the port interfaces it needs** (e.g.
  `ledger.gateway`) and imports `domain` only. Split by concern — no single `app.App`.
- **`internal/hledger`, `internal/journal`** — Interface Adapters to the `hledger` binary and
  the journal file. Import `domain` only. They satisfy the use-case ports **structurally**
  (Go implicit interfaces), so no dependency runs from use case to adapter or back;
  `cmd/konto` wires the concrete into the port.
- **`internal/domain`** — Entities; types and pure rules, no I/O, imports nothing of konto's.

Deliberate deviations from strict Clean Architecture: adapters return `domain` entities
rather than per-use-case response models (ADR 0001; `domain` is the inner form, so the
Dependency Rule still holds); no standalone Presenter (view packages format `domain` → cells
themselves — fine for one TUI); `domain` is thin because konto is a frontend and the
accounting rules live in hledger.

The alternative — a traditional layered build where each layer imports the one below, or
letting views call `hledger` directly, or one `app.App` facade — was rejected: it inverts
the Dependency Rule at the use-case/adapter boundary, couples the UI to hledger, and makes
the adapters (the test seam, ticket 11) non-isolable.

Status: accepted.
