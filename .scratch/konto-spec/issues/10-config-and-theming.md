# 10 Configuration and ANSI-16 theming

Type: grilling
Status: resolved

## Question

Decide konto's configuration and theming:

- **Config sources**: journal file path (`LEDGER_FILE` env / CLI flag / config file /
  hledger's own default search), CSV import source directories, `.rules` file locations.
  Precedence order. Is there a config file at all, and if so format + location (XDG
  `~/.config/konto/`)?
- **Which journal checks konto runs** (ticket 02): basic always; strict (`balanced`,
  `commodities`, `accounts`) default-on; opt-in (`ordereddates`, `uniqueleafnames`, `tags`,
  `recentassertions`) default-off. Config shape for the on/off set.
- **Catch-all account name(s)** for `Transaction.Uncategorized` detection (ticket 03).
- **Theming**: the handoff requires `lipgloss.Color("0")`–`"15"` ANSI indices, never hex, so
  the TUI inherits the user's terminal theme (they switch themes in Omarchy/Ghostty). Map
  the ~11 semantic roles (background, foreground, dim, selection, red, green, amber, blue,
  magenta, cyan, bright-white) to specific ANSI indices. Decide the rule for ambiguous ones
  (amber/focus → 3 or 11? dim → 8?). Is the palette user-overridable or fixed?
- Where the theme/palette lives in the package tree (feeds ticket 04 if still open).

Feeds **Assemble docs/spec.md**.

## Answer

Scoped to what the build hits first (re-scope 2026-09-10). **Resolved here:** journal-path
resolution, the ANSI-16 palette. **Deferred to a follow-up ticket when the build needs
them:** config-file format/location, the check-set on/off config shape, catch-all account
name(s) for `Transaction.Uncategorized`, CSV import source directories. `internal/config`
for v1 only resolves the journal path from flag + env + default; a config file is added
when the first setting actually needs one.

### Journal path resolution

Order: `--file` CLI flag → `$LEDGER_FILE` → (future config `journal` key) → `~/.hledger.journal`.
konto resolves to an **absolute path at startup** and passes `-f <path>` explicitly on every
hledger invocation (deterministic; the `internal/journal` text layer needs the concrete
path; `hledger files` expands the include set from there). If nothing resolves to an existing
file → first-run screen (ticket 02).

### ANSI-16 palette (`internal/ui/theme`)

Fixed map of semantic role → ANSI index. `lipgloss.Color("N")`, never hex, so the TUI
inherits the user's terminal theme (ADR 0005).

| role | ANSI | |
|---|---|---|
| background | *unset* | terminal default |
| foreground | *unset* | terminal default |
| dim — labels, borders, secondary text | `8` | bright-black |
| selection background | `8` | paired fg → `15` |
| red — negative, error | `1` | |
| green — positive | `2` | |
| amber — warning, focus, manual-source `✎` | `3` | (not `11`) |
| blue — mode indicator, forecast line | `4` | |
| magenta — rule / price directive text | `5` | |
| cyan — panel titles | `6` | |
| bright-white — emphasis, grand totals | `15` | |

Rule: normal colours `1`–`7` for standard roles; bright `8` / `15` only for de-emphasis and
emphasis. **Fixed, not user-remappable in v1** — per-role override is a later config
feature. `theme` exposes named `lipgloss.Style` values (e.g. `theme.Dim`, `theme.Negative`,
`theme.PanelTitle`, `theme.Selected`, `theme.Emphasis`), built once from the role→index map;
view/widget code uses the named styles, never raw indices.
