# 06 Overlay and popup compositing mechanics

Type: research
Status: resolved
Findings: .scratch/konto-spec/research/06-overlay-compositing.md

## Question

The which-key popup (`g` / `?`), command palette (`:`), and journal-check panel (`!`) are
bottom-anchored overlays that composite *on top of* the current screen rather than replacing
it. Bubble Tea has no z-index / layer primitive. From primary sources (Lip Gloss docs +
source, Bubble Tea docs, the named community packages):

- What does Lip Gloss offer today — `lipgloss.Place`, and any v2 layer/canvas API? Pin the
  Lip Gloss version and what it actually supports.
- Community overlay packages (e.g. `rmhubbert/bubbletea-overlay`, `charmbracelet/lipgloss`
  compositing helpers): do they support a bottom-anchored panel over arbitrary content,
  handle wide runes / ANSI styling correctly, and are they maintained?
- The manual approach: splicing rendered lines together yourself — what are the known
  pitfalls (ANSI escape handling, string width, cursor position)?
- Recommend one approach for konto, with the version constraints it implies.

Feeds **Responsive layout strategy**; the overlay decision itself is recorded when this
ticket resolves.

## Answer

Full findings + sources: `.scratch/konto-spec/research/06-overlay-compositing.md`.

**Decision: build konto on the Charm v2 stack and composite overlays with
`lipgloss.Compositor` (Lip Gloss v2). No community overlay package, no hand-rolled
line-splicing.**

- Lip Gloss **v1** (`v1.1.0`, frozen since 2025-03-13) has only the `Place*` family, which
  pads whitespace and cannot draw over content. There is no `PlaceOverlay` in Lip Gloss.
- Lip Gloss **v2** (stable `v2.0.0` 2026-02-24, pin `v2.0.6`) ships a real cell-grid
  compositor: `Layer` (content + `X/Y/Z` + `ID`), `Compositor` (`Render()`, `Hit(x,y)`),
  `Canvas`. Compositing happens in a grapheme-aware `ultraviolet` cell buffer, so ANSI/SGR
  styles, wide/CJK runes, and mid-line escapes are handled structurally — the whole reason
  to prefer it over string splicing.
- Bubble Tea v2 `Model.View()` returns a `tea.View` struct; set
  `View.Content = compositor.Render()` and `View.Cursor = tea.NewCursor(x,y)` at the
  post-composite field position. This fixes the classic v1 overlay-cursor problem (needed
  for the `:` command palette).
- konto shape: base view = one `Layer` at `X(0).Y(0).Z(0)` sized to `tea.WindowSizeMsg`;
  the single visible popup = a `Layer` at `Z(10)`, `Y(screenH - popupH - 1)`. Only one of
  `whichKeyMode` / `commandPaletteOpen` / `checksPanelOpen` is ever active, so no real
  z-stack management.

**Version pins** (v2 uses vanity import paths, not `github.com/charmbracelet/.../v2`):

```
charm.land/bubbletea/v2 v2.0.9
charm.land/lipgloss/v2  v2.0.6
charm.land/bubbles/v2   v2.2.1
```

The v1 fallback (`rmhubbert/bubbletea-overlay` + Bubble Tea v1.3.10, accepting a cursor hack)
is off the table.

**Fallback if pinned to v1:** `rmhubbert/bubbletea-overlay` `v0.6.x` — it's the tested
packaging of the string-splice algorithm (do not hand-roll); supports bottom-anchored
full-width panels; no declarative cursor.
