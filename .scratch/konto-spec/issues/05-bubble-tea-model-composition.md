# 05 Bubble Tea model composition and message routing

Type: grilling
Status: open
Blocked by: 03, 04

## Question

Decide how the Bubble Tea program is structured:

- Root model + one sub-model per view: how `Update` and `View` delegate to the active view,
  and whether inactive views keep state or are rebuilt on entry.
- How shared state is threaded: loaded journal data, `valuationMode`, `period`, `scenario`,
  terminal size, `whichKeyMode`, `checksPanelOpen`, command-palette state. Passed down by
  value, shared pointer, or broadcast as messages? (Handoff "State Management" section lists
  the suggested shape.)
- Keymap dispatch: number keys 1–6 for direct nav, the one-shot `g` goto-prefix state
  machine, `:` command line with prefix-match + Tab-complete + Enter-to-navigate, `?` / `!`
  toggles. Global keymap vs per-view keymap layering — where does a keypress get resolved?
- Which Bubbles components are used and where: `list`, `textinput`, `viewport`, `table`,
  `help`, `key`.
- How a data-refresh `tea.Cmd` (from ticket 02) delivers results back into the right models.

Feeds **Responsive layout strategy** and **Assemble docs/spec.md**.
