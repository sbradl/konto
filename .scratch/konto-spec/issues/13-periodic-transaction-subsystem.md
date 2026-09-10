# 13 Periodic transaction subsystem design

Type: grilling
Status: open
Blocked by: 02, 03

## Question

The Scheduled view does full CRUD on hledger `~` periodic rules (`n` new, `e` edit,
`p` pause, `x` delete, Enter = post the next occurrence into the real journal). hledger has
no command to list, edit, or pause `~` rules, and no "paused" concept. Decide:

- **Parsing.** konto reads `~` blocks (interval expression, postings, rule-level
  comment/tags) straight from journal text. Confirm the parser's scope and where it lives
  relative to the hledger gateway — it is shared with the `account`-directive parsing from
  ticket 03 and the `~`-for-forecast use in ticket 09. Is there one small journal-text
  parser, or several targeted extractors?
- **`PeriodicRule` fields** (named but not detailed in ticket 03): raw text, interval
  expression, parsed postings, start/end bounds, status.
- **Pause mechanism** — no hledger equivalent. Options: comment the `~` line out, a
  `; konto:paused` tag, or a konto-side list. Whichever must round-trip through `e` edit and
  not confuse `hledger check` / `--forecast`.
- **Plain-English cadence** rendering from the interval expression ("every 15th of month" →
  "monthly on the 15th").
- **Next-occurrence generation** — konto-computed from the interval, or read from
  `print --forecast -O json` filtered by source position (ticket 01: generated txns carry
  `_generated-transaction` tag + `tsourcepos` of the `~` line).
- **Post-next-occurrence write path** — which file the concrete transaction is appended to,
  how the date is chosen, how it coordinates with the gateway's read cache (ticket 02).
  Shared with the Entry screen's manual-commit write path and the account-classification
  write from ticket 03.

Feeds **Assemble docs/spec.md**; clears most of the "journal writes beyond CSV import" fog.
