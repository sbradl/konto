# 07 Responsive layout strategy

Type: prototype
Status: open
Blocked by: 06

## Question

The mock is a fixed 180×60 grid; the real app must recompute all layout from
`tea.WindowSizeMsg`. Decide the strategy:

- A shared layout helper (column-split ratios, min panel sizes, box-drawing math, title
  insets) that every view uses, vs ad-hoc Lip Gloss composition per view.
- Degradation rules for narrow / short terminals — the handoff leaves this open. What's the
  minimum supported size? Do the dashboard's three columns stack, drop panels, or scroll?
- How overlays reserve their rows against the live layout. Ticket 06 decided the mechanism:
  `lipgloss.Compositor` (Charm v2), base view as Z0 layer, single popup as Z10 layer at
  `Y(screenH - popupH - 1)`. This ticket decides how popup height/width is computed from
  the live `WindowSizeMsg`.
- Confirm `ntcharts` braille line chart handles the dual-series actual+forecast join and
  thousands y-axis the Forecast and Dashboard charts need — **and** that `ntcharts/v2`
  (`v2.2.0`) builds against real `charm.land/bubbletea/v2` (its `go.mod` carries an ignored
  `replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 ...`). A build
  failure here forces reconsidering the v1 stack (see ticket 06 answer).

**Prototype**: rough Lip Gloss layout of the Dashboard's three-column grid at ~3 widths
(wide, laptop, narrow) plus a stub braille chart, to react to before locking the strategy.

Feeds **Assemble docs/spec.md** and the "per-view implementation notes" fog.
