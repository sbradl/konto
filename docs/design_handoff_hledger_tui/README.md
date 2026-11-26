# Handoff: hledger TUI (Bubble Tea)

## Overview
A terminal UI (TUI) frontend for [hledger](https://hledger.org), a plain-text accounting tool. Single user, personal finance. Four views: dashboard, CSV import with rule-building, manual entry, and scheduled/periodic transactions. Navigation is vim-style: number keys jump views, `j`/`k` move a cursor, `:` opens a command line, `g` is a goto-prefix which-key menu, `?` lists all keys.

## About the design file
The bundled file, `hledger_tui_v2_cell_grid.dc.html`, is a **design reference built in HTML/JS, not code to port**. It works by painting into a plain 2D character array (a `grid()` of `{c, f, b, o}` cells — character, foreground, background, bold) and rendering that array as one `<div>` per row, one `<span>` per style run. This is a simulation of a terminal cell buffer, chosen specifically so the layout maps 1:1 to real terminal-rendering constraints (fixed cell grid, box-drawing border characters, no sub-pixel positioning, no CSS layout).

**The task is to build this as an actual Bubble Tea (Go) application** using Lip Gloss for styling and the Bubbles component library, not to embed or wrap the HTML.

## Fidelity
High-fidelity for layout, color intent, and information hierarchy. Low-fidelity for exact pixel/cell positions — the HTML mock uses a 180×60 character grid at 13px/16px cell size to fit a large monitor; a real terminal window will be whatever size the user's terminal is, so **all layout must be responsive to `tea.WindowSizeMsg`**, recomputed from actual terminal columns/rows, not hardcoded to 180×60. Treat the mock's proportions (column split ratios, box heights) as the target, not literal coordinates.

## Screens / Views

Global chrome, present on every screen:
- **Tab bar** (row 0): four tabs, ` 1 dash │ 2 import │ 3 entry │ 4 sched `. Active tab: bold white text on `sel` background. Inactive: `dim` foreground, no background. Right-aligned: journal path, transaction count, valuation mode (`at cost` / `-V market`), a `● clean` health indicator (green dot when `hledger check` passes).
- **Horizontal rule** (row 1): a full-width `─` line in `dim`.
- **Status bar** (bottom row): mode indicator ` NORMAL ` (blue bg) or ` PENDING g ` (yellow bg, when the goto-prefix is armed) in bold on dark text; current screen name; a context-sensitive hint string (see `HINTS` map in the source, e.g. `j/k row   c categorize   e edit   v valuation   ! journal checks`); a position counter `n/total`; `? keys` at the far right.
- **Command line** (`:`): replaces the status bar with `:<typed text>` and a blinking block cursor. Typeahead-filters a command list (`CMDS` in source); Tab autocompletes to the top match; Enter navigates to that command's screen.
- **Which-key popup** (`g`, or `?`): a bordered panel that slides up from the bottom, listing keys in a 4-column grid. `g` shows a "goto" submenu (`gd` dashboard, `gi` import, etc); `?` shows the full root keymap.
- **Journal-check panel** (`!`): a bordered panel from the bottom listing `hledger check --strict` output — one row per issue, severity tag (`ERR`/`WARN`/`OK`) in red/yellow/green, message, and a `file:line` reference right-aligned.

### 1. Dashboard
Purpose: at-a-glance net worth, recent activity, what's coming up.
Layout: three stat cards across the top (Net Worth, Cash Flow, Journal Checks). Below, a left column (Accounts tree, Budget bars, Market Prices, Journal Checks list) and two right columns (Recent transactions list, Upcoming scheduled items + a monthly-spend trend chart).
- **Stat cards**: bordered box, small dim label, large bold value, smaller dim note to the right of the value. Net Worth card's value/label change with valuation mode (see Design Tokens > Valuation below).
- **Accounts panel**: indented tree (2 cells per depth level), account name, a source glyph (`✎` amber = manually entered, `↓` dim = from CSV import, blank = it's a parent/header account), right-aligned balance. Negative balances in red.
- **Budget panel**: account name, a block-character progress bar (`█` filled / `░` empty, 22 cells wide, green normally, red if over 100%), percentage.
- **Market Prices panel**: hledger `P` price directives, magenta directive text, right-aligned rate.
- **Recent panel**: selectable list (`j`/`k` moves a highlighted row, `sel` background). Date, a `?` marker in yellow for uncategorized transactions, description, category (dim, or amber "uncategorized" if none), amount (green if income).
- **Upcoming panel**: same row shape as Recent but not selectable — generated from scheduled transactions, doesn't affect any balance until it actually posts.
- **Monthly spend chart**: a line chart using braille characters for sub-cell resolution (2×4 dots per terminal cell), y-axis labels in thousands, x-axis month labels.

### 2. Import
Purpose: bring in bank CSVs, review/accept matches, build categorization rules, avoid re-importing rows already seen.
Layout: left sidebar (file list + dedup explainer + keys), center (the CSV row table), right column (rule editor for the selected row + posting preview + a "skipped as already imported" list).
- **File list**: status dot per file (green = up to date, yellow = has new rows, red = never imported), filename, a meta line (e.g. "9 new rows").
- **CSV table**: match indicator (`✓` green / `?` yellow), date, description, target account (dim if matched, amber "← needs a rule" if not), amount, the rule ID that matched it.
- **Rule editor**: shows the `if <pattern> / account2 <target> / comment <text>` rule that would be written to the `.rules` file for the selected row, plus match-count stats (matches in this file / across the whole journal / existing rule count / how many rows remain unmatched after saving). Keys: Enter save, `t` edit target, `d` discard.
- **Posting preview**: the actual journal entry that will be written, including **a balance assertion derived from the CSV's own running-balance column** (`= -1284.19 EUR`) — this is the "free" correctness check described in Design Tokens > Dedup & assertions.
- **Skipped rows panel**: rows before the `.latest.<file>` marker, shown dim, so dedup is legible rather than a bare count.

### 3. Entry (manual transactions)
Purpose: hand-enter transactions for accounts that are never touched by CSV import (cash in EUR/PLN/CZK, informal lending). **An account is either manual or import-fed, never both** — this is a hard product rule, not just a UI default.
Layout: left sidebar (manual account list + note that imported accounts are read-only here + keys), center (the entry form), right column (live journal-text preview + similar past entries for autofill + a commodities reference).
- **Manual accounts list**: name, current balance, transaction count. Below it, dimmed, the imported accounts are listed explicitly as *not* selectable here.
- **Form**: one row per field (date, description, account, amount, commodity, category, note), label right-aligned in dim/amber-when-focused, value in bold white when focused, a blinking block cursor after the value, an inline hint per field. `Tab`/`j`/`k` move focus. A running "balances" line at the bottom shows whether the entry is currently balanced (green ✓) or not.
- **Journal preview**: exact text that will be appended to the account's journal file on commit, plus which file, which parent file includes it, and which `hledger check` rules it satisfies.
- **Similar past entries**: past transactions matching the current description, used for one-key autofill (`Tab`) — this is what makes repeated small entries (a recurring bus ticket, etc.) fast.
- **Commodities panel**: declared commodities and their latest conversion rate, with a note that declaring them is what makes `hledger check --strict` catch a typo'd currency code.

### 4. Scheduled (periodic transactions)
Purpose: recurring transactions (rent, salary, subscriptions) — hledger's `~` periodic rule syntax.
Layout: left (full list of periodic transactions), right column (the raw journal entry for the selected one, its next 6 generated occurrences, and an annual-impact summary table).
- **List**: status dot (green = active, dim pause icon = paused), description, cadence in plain English, next date, amount (green if income), target account.
- **Journal entry panel**: the literal `~` rule and its postings, magenta for the `~` line, dim for comments.
- **Next occurrences**: dates generated from the cadence, not written anywhere until posted.
- **Annual impact table**: every active schedule × its annual frequency = per-year total, summed to a net figure at the bottom. This is meant to answer "what does my recurring commitment load look like over a year," which the row-by-row list doesn't show at a glance.
- Keys: `n` new, `e` edit, `p` pause, `x` delete, Enter posts the next occurrence into the real journal now.

## Interactions & Behavior
- All navigation is keyboard-only by design (no mouse target sizing needed), matching the "vim-like" brief: number keys 1–4 switch screens directly; `j`/`k`/arrows move the row cursor within a screen; `g` arms a one-shot goto-prefix (next key picks a screen, Escape cancels); `:` opens a command line with prefix-matching + Tab-complete + Enter-to-navigate; `?` toggles the full keymap; `!` toggles the journal-check panel; `v` toggles valuation mode; `Tab` moves between form fields on the Entry screen.
- No animations — a TUI redraws instantly on state change; there is no transition to design.
- Popups (which-key, command palette, journal checks) are bottom-anchored overlays that occupy their own rows, compositing on top of the current screen's content rather than replacing it — in a real Bubble Tea app this must be done by splicing rendered lines together yourself (Bubble Tea has no z-index/layer primitive; Lip Gloss v2 / community overlay packages can help).

## State Management
Suggested state shape (mirrors the mock's `Component.state`):
- `screen`: active view id
- `cursor` per list-bearing screen (dashboard/import/scheduled each need their own selected-row index)
- `formField`: focused field index on the Entry screen
- `valuationMode`: `native` | `market`
- `commandPaletteOpen` + `commandText`
- `whichKeyMode`: none | `root` | `g`
- `checksPanelOpen`
Real data needed behind all of this: parsed hledger journal (accounts, transactions, balances), CSV import staging state + `.latest.<file>` markers + `.rules` file, periodic transaction definitions, price directives, and the output of `hledger check --strict`. These should come from shelling out to the `hledger` CLI (or a Go port of its journal parser) rather than reimplementing hledger's own logic.

## Design Tokens

### Palette (ANSI-16 / Tokyo Night)
Use `lipgloss.Color("N")` ANSI indices (0–15) in the real build, not literal hex — that makes the TUI inherit whatever terminal theme the user has active (this was an explicit requirement: the user runs Omarchy/Ghostty and switches themes). Hex values below are the mock's stand-in for a Tokyo Night terminal theme:
- Background: `#16161e`
- Foreground (default text): `#a9b1d6`
- Dim (labels, borders, secondary text): `#414868`
- Selection background: `#292e42`
- Red (negative/error): `#f7768e`
- Green (positive/success): `#9ece6a`
- Yellow/Amber (warning/manual-source/focus): `#e0af68`
- Blue (mode indicator): `#7aa2f7`
- Magenta (rule/price directive text): `#bb9af7`
- Cyan (panel titles, cyan headers): `#7dcfff`
- Bright white (emphasized values): `#c0caf5`

### Typography
Single monospace family throughout — **a TUI cannot mix fonts or sizes**; the terminal emulator, not the app, determines the typeface (Cascadia Mono/Code is what the mock assumes, matching the user's stated preference). All hierarchy comes from **bold, color, and the ANSI palette only** — never font-size or font-family changes. The mock's cell size (13px / 16px line-height) is arbitrary presentation scale; a real terminal's cell size is whatever the user's font/zoom produces.

### Borders
Box-drawing characters only: `╭ ─ ╮ │ ╰ ╯`. In Lip Gloss, `lipgloss.RoundedBorder()`. Every border occupies a full character cell — there are no fractional-width borders, no CSS-style border-radius curves.

### Spacing
All padding/gaps are whole character cells (columns/rows), never fractional. Panel titles are inset 2 cells from the left border, e.g. `╭─ ACCOUNTS ─────╮`.

### Valuation
Two toggle-able display modes on the Dashboard:
- **At cost** (`native`, default): amounts shown in their original posted commodity. An account/total holding more than one commodity (this user holds EUR, PLN, CZK cash) cannot collapse to a single number — hledger reports it as multiple lines/amounts, and the mock represents that as `85748.24 + 2 cur` with a footnote.
- **Market** (`-V` / `--value=end`): every commodity converted to EUR using the latest `P` price directive. The gap between the two totals is unrealized exchange-rate gain/loss, not spending — call this out in UI copy, don't just show a bigger number.

### Journal checks
Maps to `hledger check --strict`. Three severities: `ERR` (red — unbalanced transaction, failed balance assertion), `WARN` (yellow — undeclared account, out-of-order dates, duplicate leaf name), `OK` (green, dim text — passing checks, shown for reassurance not action). Each row carries a `file:line` reference; clicking/selecting one should be able to jump to that location in `$EDITOR`.

### CSV dedup
Each import source file has a `.latest.<file>` marker recording the last-seen row (hledger's own de-dup convention). Reimporting the same file skips everything at or before that marker. The UI must surface this as a legible list of skipped rows, not just a count, and must offer a manual override (`d` mark a row a genuine duplicate anyway / `u` undo and re-import it) for the rare case the automatic marker is wrong.

### Balance assertions from CSV
Most bank CSV exports include a running-balance column. On import, write a `= <balance>` assertion on the last posting using that column — this catches a dropped row or a bad rule change immediately, and needs no separate "reconciliation" feature (the user confirmed CSV-only accounts don't need manual reconciliation, since the bank's export already **is** the authoritative statement).

## Assets
No image/icon assets — everything is Unicode text (box-drawing, braille, a handful of glyphs: `✎` manual-entry marker, `↓` import marker, `●`/`⏸` status dots, `✓`/`?` match markers).

## Files
- `hledger_tui_v2_cell_grid.dc.html` — the full four-screen interactive mock described above. Open it directly in a browser; it's a single self-contained file with inline JS (no build step). The earlier, less terminal-accurate iteration (proportional CSS spacing instead of a real character grid) is not included in this handoff — this v2 file supersedes it.
