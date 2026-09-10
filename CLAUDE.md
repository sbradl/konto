# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Current state

Greenfield. No Go code, no `go.mod`, no commits yet. The repo currently holds only the design handoff (`docs/design_handoff_hledger_tui/`) and workflow skills (`.claude/skills/`). The task is to build the application described in that handoff.

## What this is

`konto` — a terminal UI frontend for [hledger](https://hledger.org) (plain-text accounting), single-user personal finance. Six views: dashboard, CSV import with rule-building, manual entry, scheduled/periodic transactions, forecast, reports (balance sheet + income statement). Vim-style keyboard-only navigation.

## Target stack

- **Bubble Tea** (Go) for the app, **Lip Gloss** for styling, **Bubbles** for components.
- **`NimbleMarkets/ntcharts`** for braille line/area charts — do not hand-roll the 2×4 braille dot matrix.
- Real accounting data comes from **shelling out to the `hledger` CLI** (or a Go port of its journal parser). Do not reimplement hledger's journal parsing, balance math, `check`, `--forecast`, or valuation logic.

Once initialized, standard Go tooling applies: `go build ./...`, `go test ./...`, `go test -run TestName ./pkg`, `go vet ./...`, `gofmt`.

## Design source of truth

`docs/design_handoff_hledger_tui/README.md` is the spec — read it in full before implementing any view. The bundled `hledger_tui_v2_cell_grid.dc.html` is a **browser mock, not code to port**: it paints into a simulated terminal cell buffer so the layout maps 1:1 to real terminal constraints. Consult it for the `CMDS`, `HINTS`, and keymap tables its inline JS defines, and for layout proportions — never for literal coordinates.

## Architecture constraints from the handoff

- **Responsive layout.** The mock is a fixed 180×60 grid; the real app must recompute all layout from `tea.WindowSizeMsg` (actual terminal columns/rows). Treat the mock's column-split ratios and box heights as targets, not coordinates.
- **Colors are ANSI-16 indices** via `lipgloss.Color("0")`–`"15"`, never hex. The user switches terminal themes (Omarchy/Ghostty) and the TUI must inherit the active theme. Hierarchy comes from bold + color only — a TUI cannot vary font size or family.
- **Borders / spacing** are whole character cells only. `lipgloss.RoundedBorder()` (`╭ ─ ╮ │ ╰ ╯`); panel titles inset 2 cells (`╭─ ACCOUNTS ─────╮`).
- **Popups** (which-key `g`/`?`, command palette `:`, journal-check `!`) are bottom-anchored overlays that composite on top of the current screen. Bubble Tea has no z-index — splice rendered lines together manually (or use a Lip Gloss v2 / community overlay package).
- **No animations.**

## Hard product rules (not just UI defaults)

- **An account is either manual-entry or import-fed, never both.**
- **Forecast transactions are never written to the journal** — forecasting is a read-only `hledger balance --forecast`-style projection.
- **CSV dedup** uses hledger's `.latest.<file>` marker convention. Surface skipped rows as a legible list, not a count, and provide a manual override.
- **On CSV import, write a `= <balance>` assertion** on the last posting from the bank export's running-balance column. This is the reconciliation mechanism; CSV-only accounts get no separate reconcile feature.
- **Valuation** (`v`, shared by Dashboard + Reports): "at cost" keeps multi-commodity totals as multiple amounts (`85748.24 + 2 cur`) because hledger won't add EUR+PLN+CZK; "`-V` market" converts to EUR via latest `P` directive. UI copy must explain the delta is unrealized FX gain, not spending.

## Suggested Bubble Tea model state

`screen`, per-list `cursor` (dashboard/import/scheduled each independent), `formField`, `valuationMode` (`native`|`market`), `period` index, `scenario` index, `commandPaletteOpen`+`commandText`, `whichKeyMode` (`none`|`root`|`g`), `checksPanelOpen`. Behind it: parsed journal, CSV import staging + `.rules`, periodic transactions, price directives, `hledger check --strict` output.

## Workflow

Development is expected to be test-first. Skills in `.claude/skills/`:
- `tdd-loop` (drives `tdd-red` → `tdd-green` slice by slice, with a `craft-review`-fed `tdd-refactor` at each batch boundary).
- `craft-review` — five-lens diff review (DDD, code smells, Clean Code, Pragmatic Programmer, Philosophy of Software Design).
