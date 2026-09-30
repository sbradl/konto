# 07 Responsive layout strategy

Type: prototype
Status: resolved
Blocked by: 06

<!-- Re-scoped 2026-09-10: done DURING the shell build, not before it. The prototype IS the
first slices of the app shell (tab bar + status bar + one panel at ~3 widths). Resolve this
ticket once the shell build has settled the layout-helper shape. -->


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

## Answer

Grilled 2026-09-30 (strategy settled before the shell build; the shell build tunes numbers).
Spec: `docs/spec.md` §6.8, §6.6, §1.

1. **Floor ≈ 100×30**, global. Below → full-screen "terminal too small — need ≥W×H, have
   w×h", only `q` live.
2. **Degradation = priority drop**: shrink proportionally to minimums, then hide
   lowest-priority panels, both axes; each view declares its priority order. No whole-view
   scroll, no stacking (rejected: stack+scroll fights `j`/`k` cursors; shrink-only truncates
   into uselessness).
3. **`ui/layout`**: pure `Allocate(total, []Slot{Weight, Min, Priority}) []int` (0 =
   dropped), nested by views (columns, then per-column heights) + `Panel(title, w, h, body)`.
   Rejected: declarative layout tree (engine-building), ad-hoc per view (untestable).
4. **Popups**: full width, height = content capped at ½ screen, anchored
   `Y(screenH−popupH−1)`; popup reports natural size, root clamps. Checks list + command
   dropdown scroll; which-key reflows to `min(4, floor(w/minColW))` columns.
5. **Hidden panels**: dim status-bar note `N panels hidden · widen terminal`; no zoom key.
6. **Stack check passed**: trivial `timeserieslinechart` program builds against real
   `charm.land/bubbletea/v2 v2.0.10`, `lipgloss/v2 v2.0.6`, `ntcharts/v2 v2.2.0` — the
   ignored `replace` is harmless; v1 fallback dropped. API fit: named datasets
   (`PushDataSet`, `SetDataSetStyle`, `DrawBrailleDataSets`) → one line per account
   (ticket 09 made history+forecast one continuous series per account, so no "join"
   needed); `WithYLabelFormatter` → thousands labels.

**Deferred to the shell build** (implementation): exact floor numbers, per-view priority
orders, gap cells, `Allocate` remainder distribution + tie-break, visual check of the
forecast-start vertical line and shared-braille-cell colour.
