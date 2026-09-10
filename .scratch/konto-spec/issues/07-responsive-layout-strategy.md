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
- How overlays (ticket 06) reserve their rows against the live layout.
- Confirm `ntcharts` braille line chart handles the dual-series actual+forecast join and
  thousands y-axis the Forecast and Dashboard charts need.

**Prototype**: rough Lip Gloss layout of the Dashboard's three-column grid at ~3 widths
(wide, laptop, narrow) plus a stub braille chart, to react to before locking the strategy.

Feeds **Assemble docs/spec.md** and the "per-view implementation notes" fog.
