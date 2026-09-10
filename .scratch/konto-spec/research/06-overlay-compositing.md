# 06 – Overlay & popup compositing mechanics

Research for konto's bottom-anchored overlays (which-key `g`/`?`, command palette `:`,
journal-check `!`) that must composite **on top of** the current full-screen view.

Date: 2026-09-10. All version facts checked against GitHub source / release pages on that
date. Primary sources only (Charm repos + the community package's own source).

---

## TL;DR / Recommendation

**Build konto on the Charm v2 stack and use `lipgloss.Compositor` (Lip Gloss v2) for
overlays. Do not use a community overlay package and do not hand-roll line-splicing.**

Version constraints this implies:

| Module | Import path (v2 uses a vanity path) | Pin |
|---|---|---|
| Bubble Tea | `charm.land/bubbletea/v2` | `v2.0.9` (2026-08-19) |
| Lip Gloss | `charm.land/lipgloss/v2` | `v2.0.6` (2026-08-11) |
| Bubbles | `charm.land/bubbles/v2` | `v2.2.1` (2026-08-24) |
| ntcharts | `github.com/NimbleMarkets/ntcharts/v2` | `v2.2.0` — **see caveat §5** |

Rationale in §4; caveats in §5.

---

## 1. What Lip Gloss offers today

### 1.1 Versions (checked 2026-09-10)

- Latest **v1**: `v1.1.0`, published **2025-03-13**. No v1 release since. v1 is
  effectively frozen (bug-fix only; Bubble Tea v1's last tag `v1.3.10` is 2025-09-17).
  Source: `https://github.com/charmbracelet/lipgloss/releases`,
  `https://api.github.com/repos/charmbracelet/lipgloss/releases/tags/v1.1.0`.
- **v2 is released and stable**: `v2.0.0` published **2026-02-24**, current patch
  **`v2.0.6`** published **2026-08-11**. Betas ran `v2.0.0-beta.1` (2025-03-26) →
  `beta.3`; alphas before that.
  Source: `https://github.com/charmbracelet/lipgloss/releases`,
  `.../releases/tags/v2.0.0` (`published_at: 2026-02-24T15:33:59Z`).
- v2 module path is `charm.land/lipgloss/v2` (vanity import), not
  `github.com/charmbracelet/lipgloss/v2`. Source: `go.mod` on `master`
  (`module charm.land/lipgloss/v2`).

### 1.2 Lip Gloss v1 — `Place` family only, NO compositing

v1 exposes `Place`, `PlaceHorizontal`, `PlaceVertical` (and `WithWhitespaceChars` /
`WithWhitespaceForeground`). These position a block **inside an empty box of a given
size** — they pad with whitespace (or your whitespace chars). They do **not** draw over
existing content: there is no "background string" parameter. There is **no `PlaceOverlay`
in Lip Gloss** (that name belongs to community code, see §2).
Source: `https://raw.githubusercontent.com/charmbracelet/lipgloss/master/README.md`
("Placing Text in Whitespace" section: *"Sometimes you’ll want to place a block of text
in whitespace."*). v1 has no `layer.go` / `canvas.go` (tag `v1.1.0` file listing).

So on **v1 the only way to get a true overlay is manual line-splicing or a community
package** (§2, §3).

### 1.3 Lip Gloss v2 — real cell-based compositor

v2 ships `Layer`, `Compositor`, and `Canvas`. Files:
`https://github.com/charmbracelet/lipgloss/blob/v2.0.6/layer.go` and `.../v2.0.6/canvas.go`.

Introduced in `v2.0.0-beta.2`: release notes state *"The big news in this release is
compositing."* (`.../releases/tag/v2.0.0-beta.2`). API was reshaped between beta and
stable (beta used `NewCanvas(layers...)`; stable splits `Canvas` and `Compositor`).

**`Layer`** (`layer.go`) — pure data:
```go
func NewLayer(content string, layers ...*Layer) *Layer   // content = a rendered/styled string
func (l *Layer) X(x int) *Layer                           // position relative to parent
func (l *Layer) Y(y int) *Layer
func (l *Layer) Z(z int) *Layer                           // z-index relative to parent
func (l *Layer) ID(id string) *Layer                      // for hit-testing / lookup
func (l *Layer) AddLayers(layers ...*Layer) *Layer        // nest children
func (l *Layer) Width() int / Height() int
func (l *Layer) Draw(scr uv.Screen, area uv.Rectangle)    // content -> uv.NewStyledString(...).Draw
```

**`Compositor`** (`layer.go`) — flattens a layer tree, sorts by absolute z, draws:
```go
func NewCompositor(layers ...*Layer) *Compositor
func (c *Compositor) AddLayers(...) *Compositor
func (c *Compositor) Refresh()                            // re-flatten after moving layers
func (c *Compositor) Hit(x, y int) LayerHit               // top-most layer ID at a point (mouse)
func (c *Compositor) GetLayer(id string) *Layer
func (c *Compositor) Bounds() image.Rectangle
func (c *Compositor) Render() string                      // -> temp Canvas -> string
```
Drawing order: `slices.SortFunc(... a.layer.z - b.layer.z)` then draw low→high, so
**"later / higher-Z layers appear on top"** (doc comment on `Canvas`). `Hit` walks
highest-Z first, returns the first layer whose bounds contain the point and whose `id != ""`.

**`Canvas`** (`canvas.go`) — a fixed-size `uv.ScreenBuffer` cell buffer:
```go
func NewCanvas(width, height int) *Canvas   // c.scr.Method = ansi.GraphemeWidth
func (c *Canvas) Compose(drawer uv.Drawable) *Canvas
func (c *Canvas) Render() string             // uv.TrimSpace(c.scr.Render())
func (c *Canvas) CellAt / SetCell / Resize / Clear ...
```

**How it handles ANSI + wide runes:** `Layer.Draw` does
`uv.NewStyledString(l.content).Draw(scr, area)` — the content string is **parsed** by
`charmbracelet/ultraviolet` into styled cells (SGR attributes become cell styles,
hyperlinks preserved), and written into a cell buffer whose width method is
`ansi.GraphemeWidth` (grapheme-cluster / East-Asian-wide aware). Because compositing
happens **in the cell grid, not on the raw string**, split wide runes, mid-line escape
sequences, and style bleed are handled structurally — this is the whole point of the v2
compositor and the reason to prefer it over §3.
Source: `layer.go:153-156`, `canvas.go:22-27`
(`c.scr.Method = ansi.GraphemeWidth`), `go.mod` requires
`github.com/charmbracelet/ultraviolet` and `github.com/charmbracelet/x/ansi v0.11.6`.

**Caveat I could not fully verify from source in the time available:** `Canvas.Render()`
calls `uv.TrimSpace(...)`, which trims trailing whitespace per line. For a full-screen
alt-screen TUI this is normally fine (the renderer pads), but if konto ever needs exact
trailing-cell background fills behind the popup, test this. Also the base layer must be a
full W×H block (pad it with Lip Gloss `Width/Height`) or `Compositor.Bounds()` — the union
of layer bounds — can come out smaller than the screen.

### 1.4 Bubble Tea v2 integration point

Bubble Tea v2 `Model.View()` returns a `tea.View` struct (not a bare string):
`Content string`, `Cursor *tea.Cursor`, `OnMouse func(MouseMsg) Cmd`, `AltScreen bool`,
plus background/foreground color, window title, progress bar.
Source: `https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go` (`type View struct`,
`func NewView`, `func (v *View) SetContent`).

For konto: `View.Content = compositor.Render()`, and when the command palette / an
entry field is focused, set `View.Cursor = tea.NewCursor(absX, absY)` at the field's
**post-composite** screen position. This removes the classic manual-splice cursor problem
(§3) — in v1 you cannot place the cursor declaratively, in v2 you can.

---

## 2. Community overlay packages

### 2.1 `rmhubbert/bubbletea-overlay` — **v1 only, feature-frozen**

Source: `https://github.com/rmhubbert/bubbletea-overlay`, `composite.go`, `CHANGELOG.md`,
`go.mod` (all on `main`, checked 2026-09-10).

- **Purpose:** wraps a `background` and a `foreground` model, composites `foreground.View()`
  onto `background.View()` at render time.
- **Deps (`go.mod`):** `github.com/charmbracelet/bubbletea v1.3.10`,
  `github.com/charmbracelet/lipgloss v1.1.0`, `github.com/charmbracelet/x/ansi v0.11.8`.
  **Bubble Tea/Lip Gloss v1 only.**
- **Maintenance:** latest tag `v0.6.x` (CHANGELOG: `v0.6.0` 2025-12-15). Recent commits
  (2026-08) are **all Dependabot bumps** — no feature work. The README carries a note
  (dated 2026-02-28 per the release page) telling v2 users to use Lip Gloss v2's built-in
  compositing instead; the author will keep v1 working but **won't add features**.
- **Bottom-anchored panel over arbitrary content:** **Yes.** `Position` enum
  `Top/Right/Bottom/Left/Center` on each axis + integer `X`/`Y` offsets. `Bottom` computes
  `y = lipgloss.Height(bg) - lipgloss.Height(fg)` (`composite.go` `offsets()`), then
  `clamp`s into range. Bottom-anchored full-width panel is exactly what it does well.
- **ANSI styling:** **Handled** via `github.com/charmbracelet/x/ansi`:
  `ansi.Truncate`, `ansi.TruncateLeft`, `ansi.StringWidth` are used to cut the background
  line around the foreground (`composite.go` `Composite()`). `x/ansi` truncation is
  SGR-aware (re-emits/΅resets styles at cut points).
- **Wide / CJK runes:** **Partially.** Width math goes through `ansi.StringWidth` /
  `lipgloss.Width` / `lipgloss.Size`, which are `go-runewidth`/`uniseg`-based, so column
  math is correct. `ansi.Truncate` in current `x/ansi` replaces a wide rune straddling the
  cut with a space. The residual risk is the algorithm assuming every `fgLine` is exactly
  `fgWidth` columns — a ragged foreground (lines of unequal width) misaligns the spliced
  right-hand background segment. In practice you always render the popup through a
  Lip Gloss style with a fixed `Width(...)`, which pads every line, so this is a
  non-issue if you're disciplined.
- **Algorithm origin:** `composite.go` header comment: *"based off of the one used by
  Superfile — https://github.com/yorukot/superfile/.../overplace.go"*. So this is the
  reference implementation of the manual approach, packaged and tested
  (`composite_test.go`, unit tests for `clamp`/`lines`/`offsets`/`Composite`).
- **No cursor handling** — inherent to Bubble Tea v1 (see §3).

### 2.2 Other packages found

- Forks of the above: `quickphosphat/bubbletea-overlay`, `jsdoublel/bubbletea-overlay`
  — same code, no reason to prefer.
- `sraaaaaaay/bubbletea-modal/v2` — a thin wrapper around `lipgloss.NewLayer()` for the v2
  stack. If you're on v2 you already have the compositor; a wrapper adds a dependency for
  a few lines of glue. (Pseudo-version only, `v2.0.0-2026032...`; low adoption. Not
  recommended.)
- `lrstanley/bubblezone` (v2 exists, `v2.0.0`, used transitively by ntcharts) — **mouse
  "zone" tracking, not overlay compositing.** Useful later if konto wants clickable
  regions; irrelevant to the overlay-drawing question. konto is keyboard-first anyway.
- Charm has **no** separate compositing helper package — it lives inside `lipgloss/v2`.

**Verdict on community packages:** `rmhubbert/bubbletea-overlay` is the right choice
*only if konto is pinned to the v1 stack*. On v2 it is strictly worse than the built-in
`lipgloss.Compositor` (extra dep, string-splice algorithm vs. cell-grid, no upstream
feature support).

---

## 3. The manual line-splicing approach — concrete pitfalls

If you did it by hand (v1, no package), you would reimplement `composite.go`. The known
hazards, each of which that file has had to address:

1. **ANSI escape sequences mid-line.** You cannot `str[x:]` a rendered line — byte index
   ≠ column. Escape sequences (`\x1b[38;5;12m` …) occupy bytes but zero columns. You must
   use a column-aware cutter (`ansi.Truncate`, `ansi.TruncateLeft`, or `muesli/reflow`).
2. **Style bleed across the splice.** When you cut the background line and drop the
   foreground in, the background's *active* SGR state at the cut column leaks into the
   foreground (and the foreground's trailing state leaks into the resumed background).
   Fix: the cutter must emit a reset (`\x1b[0m`) at each seam and re-open the background's
   style when it resumes. `x/ansi`'s truncators do this; a naive `strings` cut does not.
3. **String width vs byte length.** `len(s)` is bytes; you need display columns
   (`lipgloss.Width` / `ansi.StringWidth` / `uniseg`). Off-by-one here shifts every line
   after the popup.
4. **Rune width (wide / CJK / emoji / combining).** A CJK glyph or emoji is 2 columns;
   combining marks and ZWJ sequences are 0 or form one grapheme. Cutting *inside* a wide
   rune or grapheme cluster corrupts the line — the cutter must round to grapheme
   boundaries and pad the freed half-cell with a space.
5. **Ragged foreground lines.** The splice math assumes `fgLine` width == popup box
   width on every row. Always render the popup through `lipgloss.NewStyle().Width(w)` so
   every line is padded to `w`.
6. **Cursor position (Bubble Tea v1).** v1's `View() string` gives you no way to say
   where the cursor is — the terminal leaves it wherever the last byte landed. A text
   input inside an overlaid popup will show its cursor at the wrong place (bottom-right of
   the whole frame). Workarounds in v1: hide the cursor and fake a block/underline cell,
   or use `bubbles/textinput` in a mode that renders its own cursor glyph. **Bubble Tea
   v2 fixes this** with `tea.View.Cursor` (§1.4).
7. **`\r\n` / trailing newline normalisation** — `composite.go` `lines()` does
   `strings.ReplaceAll(s, "\r\n", "\n")` before splitting. Miss this and Windows-ish
   content mis-splits.
8. **Hyperlinks (OSC 8) and other OSC sequences** — rarer, but a dumb cutter can split an
   `\x1b]8;;URL\x07` sequence. Cell-grid compositing (v2) preserves these; string
   splicing needs care.

**Reference code for doing it right:**
- `charmbracelet/lipgloss/v2` `layer.go` + `canvas.go` (the cell-grid way — the gold
  standard).
- `rmhubbert/bubbletea-overlay/composite.go` (the string-splice way, tested).
- `yorukot/superfile` `src/pkg/string_function/overplace.go` (the original string-splice
  algorithm rmhubbert adapted).
- `charmbracelet/x/ansi` `Truncate` / `TruncateLeft` / `Cut` — the primitives any
  string-splice approach must build on.

---

## 4. Recommendation for konto

**Adopt the Charm v2 stack now and use `lipgloss.Compositor`.**

Why:

1. **konto is greenfield.** `CLAUDE.md`: *"Greenfield. No Go code, no `go.mod`, no commits
   yet."* There is zero migration cost to starting on v2. The only reason to pick v1 would
   be an ecosystem dependency that lags — and konto's one non-Charm UI dep, `ntcharts`,
   **has a v2 line** (`github.com/NimbleMarkets/ntcharts/v2`, tag `v2.2.0`; default branch
   is now `v2`).
2. **v1 is frozen, v2 is where maintenance happens.** Lip Gloss v1 last shipped
   2025-03-13; Bubble Tea v1 last shipped 2025-09-17. Lip Gloss v2 shipped a patch
   2026-08-11, Bubble Tea v2 2026-08-19, Bubbles v2 2026-08-24.
3. **The compositor solves the exact problem correctly.** Full-screen base view = one
   `Layer` at `X(0).Y(0).Z(0)` sized to the `tea.WindowSizeMsg`. Each popup = a `Layer`
   with `Z(10)`, positioned bottom-anchored:
   `Y(screenH - popupH)` (optionally `- 1` for a status line), `X(0)` for full-width or
   centered. `compositor.Render()` → `tea.View.Content`. Only one popup is ever visible
   (`whichKeyMode`, `commandPaletteOpen`, `checksPanelOpen` in the suggested model
   state), so a single overlay layer suffices; no real z-stack management needed.
4. **Cursor is handled declaratively** via `tea.View.Cursor` — needed for the command
   palette (`:`) and any future text entry. This is impossible in v1 without hacks (§3.6).
5. **Cell-grid compositing** means ANSI-16 theme colors (`CLAUDE.md`: colors are ANSI
   indices via `lipgloss.Color("0")`–`"15"`), `RoundedBorder()` popup frames, and any
   future wide-glyph content composite without the string-splice hazard class in §3.
6. **`Compositor.Hit(x, y)`** gives mouse-click routing for free if konto ever wants a
   clickable popup, without pulling in `bubblezone`.

Concretely, `go.mod` gets:
```
require (
    charm.land/bubbletea/v2 v2.0.9
    charm.land/lipgloss/v2  v2.0.6
    charm.land/bubbles/v2   v2.2.1
    github.com/NimbleMarkets/ntcharts/v2 v2.2.0
)
```
No overlay dependency at all — compositing is in `lipgloss/v2`.

**Fallback:** if for some reason konto must stay on the v1 stack, use
`rmhubbert/bubbletea-overlay` (`v0.6.x`, pin exact) with `bubbletea v1.3.10` /
`lipgloss v1.1.0`, accept the cursor hack for the command palette, and always render
popups through a fixed-`Width` style. Do **not** hand-roll the splice — the package is
the tested version of that code.

---

## 5. Caveats / things to verify at adoption time

- **`ntcharts/v2` `go.mod` carries a `replace` directive:**
  `replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 v2.0.0-20260506185856-6506c47fa2f3`
  with the comment `// Awaiting upstream merges`
  (source: `https://raw.githubusercontent.com/NimbleMarkets/ntcharts/v2/go.mod`,
  checked 2026-09-10). A `replace` in a *dependency's* `go.mod` is **ignored** by
  konto's build — so konto will compile `ntcharts/v2` against the real
  `charm.land/bubbletea/v2`. If that fails to build, konto would need to copy the
  `replace` into its own `go.mod` (ugly) or wait for the upstream merge. **Verify
  `go build ./...` with a trivial ntcharts chart before committing to v2**, and check
  whether a newer `ntcharts/v2` tag has dropped the replace. This is an ntcharts problem,
  not a Lip Gloss compositor problem, but it's the one thing that could push konto back
  to the v1 stack. (ntcharts `go.mod` also pins `lipgloss/v2 v2.0.3` / `bubbles/v2
  v2.1.0` as its *minimums*; konto's higher pins satisfy MVS.)
- **v2 import path is `charm.land/...`**, not `github.com/charmbracelet/.../v2`. Charm
  redirects it, but be aware when reading older docs/examples that still show the GitHub
  path.
- **`Canvas.Render()` trims trailing whitespace** (`uv.TrimSpace`). Confirm the base
  layer fills the full terminal rectangle (explicit `Width`/`Height` on the root style)
  so the popup's right edge and any full-bleed background sit where expected.
- **API churned between beta and stable** (beta `NewCanvas(layers...)` → stable
  `Compositor` + `Canvas`). Follow `examples/canvas/main.go` **at the tag you pin**
  (`https://github.com/charmbracelet/lipgloss/blob/v2.0.6/examples/canvas/main.go`), not
  the beta.2 blog snippet.
- I did **not** find a Charm doc page (charm.sh / pkg.go.dev prose) that walks through
  compositing inside a Bubble Tea `View()` end-to-end; the authoritative material is the
  `examples/canvas` program plus the `layer.go` / `tea.go` doc comments. Flag for the
  implementer: expect to read source, not a tutorial.
- Bubble Tea v2 `View.Content` is typed `string` even though the `Model.View()` doc
  comment says the view "can be a string or a `[Layer]`". Pass `compositor.Render()`.
  (`tea.go:96`, `tea.go:279`.)

---

## Sources

- Lip Gloss releases / tags / dates:
  `https://github.com/charmbracelet/lipgloss/releases`,
  `https://api.github.com/repos/charmbracelet/lipgloss/tags`,
  `.../releases/tags/v2.0.0` (2026-02-24), `.../releases/tags/v2.0.6` (2026-08-11),
  `.../releases/tags/v1.1.0` (2025-03-13), `.../releases/tag/v2.0.0-beta.2`.
- Lip Gloss v2 compositor source (tag v2.0.6):
  `https://github.com/charmbracelet/lipgloss/blob/v2.0.6/layer.go`,
  `https://github.com/charmbracelet/lipgloss/blob/v2.0.6/canvas.go`,
  `https://github.com/charmbracelet/lipgloss/blob/v2.0.6/examples/canvas/main.go`.
- Lip Gloss README (Place family; "cell-based compositor"):
  `https://raw.githubusercontent.com/charmbracelet/lipgloss/master/README.md`.
- Lip Gloss `go.mod` (module path, ultraviolet / x/ansi deps):
  `https://raw.githubusercontent.com/charmbracelet/lipgloss/master/go.mod`.
- Bubble Tea tags / dates:
  `https://api.github.com/repos/charmbracelet/bubbletea/tags`,
  `.../releases/tags/v2.0.9` (2026-08-19), `.../releases/tags/v1.3.10` (2025-09-17).
- Bubble Tea v2 `View` / `Cursor` API:
  `https://github.com/charmbracelet/bubbletea/blob/v2.0.9/tea.go`.
- Bubbles tags: `https://api.github.com/repos/charmbracelet/bubbles/tags`
  (`v2.2.1`, 2026-08-24).
- `rmhubbert/bubbletea-overlay`: `https://github.com/rmhubbert/bubbletea-overlay`,
  `https://raw.githubusercontent.com/rmhubbert/bubbletea-overlay/main/composite.go`,
  `.../main/go.mod`, `.../main/CHANGELOG.md`,
  `https://api.github.com/repos/rmhubbert/bubbletea-overlay/commits` (2026-08 = Dependabot only).
- Superfile splice origin (cited by composite.go):
  `https://github.com/yorukot/superfile/blob/main/src/pkg/string_function/overplace.go`.
- ntcharts v2: `https://api.github.com/repos/NimbleMarkets/ntcharts` (default branch `v2`),
  `.../tags` (`v2.2.0`), `https://raw.githubusercontent.com/NimbleMarkets/ntcharts/v2/go.mod`
  (bubbletea `replace` directive).
- Other overlay packages: `https://pkg.go.dev/github.com/quickphosphat/bubbletea-overlay`,
  `https://pkg.go.dev/github.com/jsdoublel/bubbletea-overlay`,
  `https://libraries.io/go/github.com/sraaaaaaay/bubbletea-modal/v2/...`.
