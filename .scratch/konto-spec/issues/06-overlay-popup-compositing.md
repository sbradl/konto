# 06 Overlay and popup compositing mechanics

Type: research
Status: claimed
Context: /research subagent running (worktree-isolated); findings -> .scratch/konto-spec/research/06-overlay-compositing.md

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
