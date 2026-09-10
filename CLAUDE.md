# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Read first

- **`docs/spec.md`** — the settled architectural foundation. Read it in full before writing
  code. Stack + versions, ports-and-adapters layering, package tree, hledger integration,
  domain model, Bubble Tea composition, config/theming, testing, and the suggested build
  order all live there.
- **`CONTEXT.md`** — the domain glossary (ubiquitous language). Use these terms.
- **`docs/adr/`** — the hard-to-reverse decisions (0001 rich domain model, 0002 flat
  account model, 0003 ports-and-adapters, 0004 root-mediator Bubble Tea model, 0005 fixed
  ANSI-16 palette).
- **`docs/design_handoff_hledger_tui/README.md`** — the frozen UX spec (all six views,
  keymaps, panels, tokens). Authoritative except the deviations listed in `docs/spec.md` §11.
  The bundled `.dc.html` is a browser mock, **not code to port** — consult it for the `CMDS`
  / `HINTS` / keymap tables and layout proportions only, never literal coordinates.
- **`.scratch/konto-spec/`** — the wayfinder map and decision tickets (the "why" behind
  every choice in the spec), plus `research/` (hledger output, overlay compositing) and,
  later, `samples/` (bank CSV exports).

## What this is

`konto` — a terminal UI frontend for [hledger](https://hledger.org) (plain-text accounting),
single-user personal finance. Six views: dashboard, CSV import with rule-building, manual
entry, scheduled/periodic transactions, forecast, reports. Vim-style keyboard-only navigation.

## State

Foundation spec complete; build not yet started. `go.mod` (`github.com/sbradl/konto`,
go 1.27) is the only Go artifact. Per-feature design (CSV import, forecast, periodic
transactions) is grilled just-in-time as each view is built — see `docs/spec.md` §12 and the
open tickets in `.scratch/konto-spec/issues/`.

## Non-negotiables (full detail in docs/spec.md)

- **Charm v2 stack**, vanity import paths: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`,
  `charm.land/bubbles/v2`, `github.com/NimbleMarkets/ntcharts/v2`. Not the `github.com/charmbracelet/.../v2` paths. v1 is frozen.
- **`hledger` binary is a hard runtime dependency**, `>= 1.32`, checked at startup. konto
  shells out; it does **not** reimplement hledger's parsing, balance math, `check`,
  `--forecast`, or valuation. The one exception is `internal/journal`, a targeted
  line-reader/writer for the handful of constructs hledger exposes no machine access to
  (`~` rules, `account` directive tags, `.latest` files) — never a full parser.
- **Ports-and-adapters** (ADR 0003): dependencies point inward to `internal/domain`; use
  cases declare their own port interfaces; `internal/hledger` / `internal/journal` satisfy
  them structurally; `cmd/konto` wires. Views never import the adapters or each other.
- **Colours are ANSI-16 indices** (`lipgloss.Color("0")`–`"15"`), never hex.
- **All layout responsive to `tea.WindowSizeMsg`** — never hardcoded to the mock's 180×60.
- **An account is manual or import-fed, never both.** **Forecast is never written.** **CSV
  dedup shows skipped rows as a list.** **CSV import writes a `= <balance>` assertion.**

## Commands

- `go build ./...`, `go vet ./...`, `go test ./...`
- `go test -run TestName ./internal/pkg` — single test
- `go test -tags integration ./...` — the suite that runs the real `hledger` binary
- `golangci-lint run` (installed via mason)

## Workflow

Test-first. Skills in `.claude/skills/`:
- `tdd-loop` — drives `tdd-red` → `tdd-green` slice by slice, with a `craft-review`-fed
  `tdd-refactor` at each batch boundary.
- `craft-review` — five-lens diff review (DDD, code smells, Clean Code, Pragmatic
  Programmer, Philosophy of Software Design).

To advance a deferred per-feature ticket: `/mattpocock-skills:wayfinder work through the map, ticket NN`.
