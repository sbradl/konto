# Fixed ANSI-16 palette

konto styles everything with `lipgloss.Color("0")`–`"15"` (ANSI indices), never hex, and the
role→index map is **fixed in code** (`internal/ui/theme`), not user-configurable in v1.

The user runs Omarchy / Ghostty and switches terminal themes; using ANSI indices means the
TUI re-colours with the terminal, which hex values would defeat. A per-role override
mechanism is deliberately deferred — it adds config surface for a need that may never
arise, and the fixed map (normal colours 1–7 for standard roles, bright 8/15 only for
de-emphasis/emphasis; `background`/`foreground` left unset) is expected to read well under
any reasonable terminal theme.

Reversible: adding an override table later is straightforward and does not change the
`theme` package's public style-name API.

Status: accepted.
