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

Toolchain (go, golangci-lint, gotestsum, gremlins, hledger) is pinned in `mise.toml`;
`mise install` provisions it. `mise tasks` lists every task.

- `mise run build` — `go build ./...`
- `mise run vet` — `go vet ./...`
- `mise run lint` — `golangci-lint run`
- `mise run check` — build + vet + lint + cover:check (the CI gate)

Thresholds start maxed by user directive — loosen a threshold in `mise.toml` only with a
recorded reason, never silently.

## Testing

Verified against this repo's `mise.toml` / installed toolchain. The repo has no Go packages
yet, so the `./...` forms below were exercised through a throwaway package; substitute a
real import path (e.g. `./internal/ledger`) once packages exist.

- **Full test suite** — `mise run test` (unit, `-race`, gotestsum) ·
  `mise run test:integration` (adds `-tags integration`, shells out to real `hledger`).
  Raw equivalent: `go test -race ./...`
- **Single test by name** — `go test -run 'TestName$' ./internal/pkg`
  (or `gotestsum -- -run 'TestName$' ./internal/pkg`). Go has no file-scoped runner: run a
  whole file's worth by `-run` over its test names, or pass the files explicitly:
  `go test ./internal/pkg/foo.go ./internal/pkg/foo_test.go`
- **Build / typecheck** — `mise run build` (`go build ./...`) · `mise run vet`
  (`go vet ./...`). Single package: `go build ./internal/pkg`
- **Coverage for a single file** — profile the package, then filter the per-function report
  to that file:
  `go test -coverprofile=coverage.out ./internal/pkg && go tool cover -func=coverage.out | grep 'foo\.go'`
  Line-by-line: `go tool cover -html=coverage.out` (or `mise run cover:html`).
  Whole-module gate: `mise run cover:check` (fails under `THRESHOLD`, default **100**).
- **Mutation testing** — `mise run mutation internal/pkg` (one directory, recurses) ·
  `mise run mutation` (whole module) · `mise run mutation:diff` (only lines changed vs
  `main`). Efficacy + mutant-coverage thresholds are **100**.
  Raw equivalent: `gremlins unleash --coverpkg=./... internal/pkg`

## Workflow

Test-first. Skills in `.claude/skills/`:
- `tdd-loop` — drives `tdd-red` → `tdd-green` slice by slice, with a `craft-review`-fed
  `tdd-refactor` at each batch boundary.
- `craft-review` — five-lens diff review (DDD, code smells, Clean Code, Pragmatic
  Programmer, Philosophy of Software Design).

To advance a deferred per-feature ticket: `/mattpocock-skills:wayfinder work through the map, ticket NN`.
