# 10 Configuration and ANSI-16 theming

Type: grilling
Status: open

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
