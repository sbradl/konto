# v1 keyboard and chrome: arrows, number keys, no command line, no panel focus

The design handoff is vim-style: `j`/`k` move a cursor, `g` arms a goto prefix with a
which-key submenu, `:` opens a command line with typeahead over a `CMDS` table, and the
status bar shows a ` NORMAL ` / ` PENDING g ` mode badge. It also carries a multi-currency
valuation toggle (`v`), export (`y`) and several read-only Dashboard panels.

konto v1 instead:

- **moves with arrows** — `↑`/`↓`, `PgUp`/`PgDn`, `Home`/`End`; `Tab`/`Shift+Tab` between
  form fields; `Esc` cancels. `j`/`k` are unbound;
- **switches views with `1`–`5` only** — no `:` command line, no `g` prefix, no which-key
  goto, no `CMDS` table, no mode badge;
- has **no panel focus** — each view has at most one cursor panel (Dashboard Accounts,
  Import CSV table, Scheduled rule list) and the arrows drive it; overflowing read-only
  panels show `… N more`, Reports' two tables scroll together;
- follows the **key principle**: every key is the first letter of its action where
  possible, one letter = one action everywhere, every write asks y/n;
- is **EUR-only in the UI** — no `v` valuation toggle, no `-V`, no Market Prices panel, no
  FX footnotes (the domain keeps `MultiAmount`; a stray non-EUR amount renders `+ N cur`);
- has **no export** (`y` dropped; out of scope, not deferred), no Budget panel, no
  Dashboard Journal Checks list or stat card, no Reports drill-in, no Recent `e` edit;
- shows **journal-check status in the status bar** of every view (`● clean` /
  `● 2 err · 3 warn`), not the tab bar.

Why: the user finds `j`/`k` and modal prefixes unnatural, five views fit on number keys,
and nothing in v1 needs a second interactive panel or a command language. Every dropped
surface either duplicated another (checks list, stat card, `g`, `:`) or had no v1 data
behind it (valuation, prices, budgets).

Reversible: adding a panel-focus model, `:` or `g` later needs no change to views' data or
use cases — only `internal/ui` routing and `ui/keys`.

Status: accepted.
