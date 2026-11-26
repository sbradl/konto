# 05 Bubble Tea model composition and message routing

Type: grilling
Status: resolved
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

## Answer

See `docs/adr/0004-root-mediator-model.md`. Lives in `internal/ui` (root) + `internal/ui/<view>`
(sub-models) per ticket 04.

### Root model

A single root model (`internal/ui`) is the program's `tea.Model`. It holds:

- `appMode ∈ {firstRun, parseError, normal}` — `firstRun` and `parseError` render their own
  screen instead of the tab UI; not a seventh view.
- the **six view sub-models**, all constructed at startup and alive for the whole run — a
  tab switch only changes which one renders and receives input; cursor/scroll/draft state
  survives switching.
- **app-global state**: `valuationMode`, `period`, `scenario`, terminal size, and the
  canonical read-only `*domain.Ledger` snapshot.
- **chrome state**: active view id, tab bar, status bar.
- **overlay state**: `whichKeyMode` (`none|root|goto`), command-line (`active bool`,
  `text string`, filtered `CMDS`), `checksPanelOpen`.
- the application-service **ports** it needs (`ledgerService` to load/refresh; the checks
  service) — injected by `cmd/konto`.

Views never reference each other. All cross-view effects are mediated by the root: a view
acts → journal/state changes → root broadcasts → every view updates. (Rationale: without
this, "Import committed, so the Dashboard must refresh" needs a view-to-view reference, which
ADR 0003 forbids.)

### View sub-model interface

```go
type View interface {
    Update(tea.Msg) (View, tea.Cmd)
    View() string          // renders into its content area (size received via WindowSizeMsg)
    Title() string         // status-bar screen name
    StatusHint() string    // context-sensitive hint (handoff HINTS map)
    KeyMap() []key.Binding  // feeds ? / which-key
}
```

Each `Update` is a pure `(Msg) → (View, Cmd)` — testable with no running program. A view
holds the application-service port(s) it needs (ticket 04) and its own view-local state
(cursor, form field, scroll, generation counter). Views hold a read-only `*domain.Ledger`,
replaced wholesale on refresh, never mutated.

### Message routing (root's `Update`)

| Message class | Routed to |
|---|---|
| `tea.KeyMsg`, `tea.MouseMsg` | the open overlay if any, else the **active view only**. Global keys (`1`–`4`, `g`, `:`, `?`, `!`, `v`, `q`/`ctrl+c`) are caught by root **before** forwarding; if the active view reports a focused text input, only `Esc`/`Ctrl+C` are caught. |
| `tea.WindowSizeMsg` | root computes the content rect (window − tab bar − rule − status bar) and forwards a **content-dimension** `WindowSizeMsg` to **all six** views |
| `ledgerLoadedMsg{*Ledger, error}`, `valuationChangedMsg`, `checksLoadedMsg`, ticks | root updates its state, then **broadcasts** to all six views |
| view-specific async results (`importPreviewMsg`, …) | root forwards to the **one owning view** |

**Shared-state changes are messages, not shared pointers.** `v` → root flips
`valuationMode`, fires the re-fetch `tea.Cmd`, broadcasts `valuationChangedMsg`.

**Async races**: each async request carries a per-view monotonic generation number; a view
drops results whose generation is stale (pairs with ticket 02's `context` cancellation).

`tea.Cmd`s returned by a sub-model's `Update` bubble up and are `tea.Batch`-ed by root.

### Keymap dispatch

Declared in `internal/ui/keys` as `key.Binding` sets — one **global** set + one **per view**.
Root resolves a key against the global set first (after the overlay/text-input checks
above), else forwards to the active view, which resolves against its own set. `?` and the
which-key popup render from these binding sets (`bubbles/help`). The `g` goto-prefix is a
root state machine: `g` → `goto` mode + which-key popup; next key navigates or `Esc`/other
resets.

### Overlays

Root-owned. The `:` command-line **input** replaces the status-bar row (handoff). The
filtered-command **dropdown**, the which-key popup, and the `!` checks panel are
`lipgloss.Compositor` layers over the composed base (ticket 06). Views know nothing about
overlays; the `!` panel renders from root's held `CheckRun`.

### Bubbles / component choices

| Component | Use |
|---|---|
| `bubbles/table` | Import CSV-row grid |
| custom selectable-list widget (`ui/widgets`) | Dashboard panels (Recent, Upcoming, Accounts) — rows need glyphs + coloured amounts `bubbles/list` fights |
| `bubbles/textinput` | Entry form fields, command line, rule-editor target |
| `bubbles/viewport` | scrollable panels (journal preview, checks body) |
| `bubbles/key`, `bubbles/help`, `bubbles/spinner` | bindings, `?` render, ticket-02 refresh indicator |
| `bubbles/list` | evaluated, **not used** |

Final component fit is confirmed by the **Responsive layout strategy** prototype.

### Startup

`cmd/konto`: config → hledger version preflight → construct adapters/services → construct
root model (inject ports + config) → `tea.NewProgram(root, tea.WithAltScreen())`. Root
`Init()` returns `tea.Batch(initial ledger load, initial checks run)`; `appMode` starts
`firstRun`/`parseError`/`normal` from the load result.
