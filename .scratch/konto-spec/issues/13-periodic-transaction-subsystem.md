# 13 Periodic transaction subsystem design

Type: grilling
Status: resolved
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

## Answer

Grilled 2026-09-30. Headline: **periodic rules are forecast-only** — konto never writes an
occurrence into the journal. This dissolves the post-occurrence write path entirely.

### Scope

- **No Enter-to-post.** A `~` rule only feeds Forecast, Dashboard Upcoming and the Scheduled
  view's projections. Real transactions arrive via CSV import or Entry. Deviation from the
  handoff → ADR 0006.
- **No pause.** No `p` key, no paused state. To stop a rule: set a remaining count, delete
  it, or create a new one. (Temporary stops = end + new rule; "what if I cancel" = Forecast
  scenarios, ticket 09.) Additive later if missed.

### Reading

- `internal/journal` reads `~` blocks from journal text (the `~` line + indented lines) per
  file of the `hledger files` include set, keeping `SourcePos` and the raw block text.
  Parser scope: one targeted line-reader alongside the `account`-directive reader — not a
  period-expression parser.
- **All dates come from hledger.** One call
  `print --forecast=<today>..<today+1y> -O json`, generated txns grouped by `tsourcepos`
  (the `~` line's file:line) → per rule: next date, next 6 occurrences, 12-month sum
  (annual impact). konto never interprets period expressions. Explicit `--forecast=PERIOD`
  is required: the default horizon starts after the journal's *latest regular transaction*,
  not today.
- **Status**: "upcoming" vs "none in the next 12 months" (derived from the grouping; no
  end-date parsing).
- **Cadence column** shows the raw period expression as written (no plain-English
  translation).
- **List** sorted by next date.

### Form (`n` new, `e` edit) — in-TUI, konto writes the text

Fields:

| field | notes |
|---|---|
| next due date | on `e`, pre-filled with hledger's next occurrence, not the stored `from` |
| frequency | dropdown: `weekly`, `biweekly`, `monthly`, `quarterly`, `half-yearly` (→ `every 6 months`), `yearly` |
| remaining payments | optional count; empty = no end |
| description | required |
| amount + currency | currency = dropdown of declared commodities, pre-selected from the from-account's held commodity (fallback: first declared) |
| from / to account | existing accounts only, type-to-filter picker |

Written shape:

```
~ <freq> from <date>[ to <end>]  <description>
    <to-account>      <amount> <currency>
    <from-account>
```

- **Count → `to`**: konto pipes the candidate rule to `hledger -f - print --forecast=…`
  on stdin, takes the N-th generated date, writes `to <N-th + 1 day>` (hledger's `to` is
  exclusive). On `e`, count = occurrences hledger generates from today until `to`. konto
  does no date arithmetic beyond +1 day.
- Period expression is valid by construction — no separate validation step.
- Verified on hledger 1.52.1: unaligned `from` dates generate on that day
  (`monthly from 2026-10-15` → 15th each month; weekly/biweekly/quarterly/yearly/
  `every 3 months` same; `from 2026-01-31` clamps to month-end). **Open:** confirm the
  1.32 floor behaves identically (integration test against the minimum version).

**`e` only on rules that fit the form**, strictly:
1. period expression is exactly `<dropdown value> from <date>[ to <date>]`
2. exactly 2 postings
3. exactly one posting carries an amount; single commodity, no `@` price, no `=` assertion
   (either posting may carry it — konto normalises to the first on save)
4. no comments or tags anywhere in the block

Otherwise `e` is refused with a hint to edit the journal by hand. Save replaces the block in
place.

**`n` target file**: appended to the file containing the last existing `~` rule; if none,
the main journal file. No include management, no config.

### Delete (`x`)

Any rule (form-compatible or not), after a y/n confirmation. Removes the `~` line and its
indented lines (postings, posting comments). Comment lines *above* the `~` are kept.

### Write safety

- The user edits the journal only through konto → no compare-before-write.
- konto **reloads after every write** (own writes shift line numbers).
- **Single-instance lock**: `flock` on a lock file next to the journal at startup; a second
  konto on the same journal refuses to start. (Advisory locks can't stop other editors —
  Linux has no mandatory locking since 5.15 — but only konto needs to honour it.)

### Display

- Annual-impact total follows the global valuation toggle `v`: at cost → one total per
  currency; market → one EUR figure via hledger `-V`.
- **UI copy says "currency"**, never "commodity" (domain/code keep `Commodity`).

### `PeriodicRule` (domain)

`{Period string (raw expression), Description, Postings []Posting (raw), Raw string (block
text), Pos SourcePos, FitsForm bool (computed)}`; projections (`Occurrences []Date`,
annual impact) joined from the forecast call, not stored on the rule.
