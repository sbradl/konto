# 09 Forecast computation design

Type: grilling
Status: resolved
Blocked by: 02, 03

## Question

The Forecast view is a read-only projection — forecast transactions are never written to the
journal (hard product rule). Decide how it's computed:

- Ticket 01 found: `bal --forecast` aggregates only (forecast rows not separable); forecast
  rows ARE identifiable via the `_generated-transaction` tag on `print --forecast` /
  `aregister --forecast` json. Decide: drive the periodic part off those tagged rows, or
  generate periodic occurrences in konto from the parsed `~` rules? Can the exact `sched`
  rows and estimated `avg 6m` rows be produced/distinguished cleanly?
- The trailing-average logic: 6-month window over what exactly (all non-periodic expense
  accounts? income too?), and how it projects forward.
- The three scenarios and precisely what each mutates:
  `base`, `lean (-15% spend)`, `3-month income gap` (income for the first 3 forecast months).
- Driver tagging so the drivers panel can show `sched` vs `avg 6m` per line, and the
  "average should be editable in place" requirement — does an in-place edit re-run the
  projection in memory only?
- Chart data shape: 6 months actual (green) joined to 12 months forecast (blue).

Feeds **Assemble docs/spec.md**.

## Answer

Grilled 2026-09-30. Headline: **the forecast is current balances + scheduled transactions,
nothing else** — no trailing averages, no scenarios, no guessing. The user keeps periodic
rules up to date. Deviations from the handoff → ADR 0007.

### Model

- Projects **each open asset and liability account** separately (not a single net-worth
  line). Closed accounts excluded (see below).
- **One hledger call** (verified 1.52.1):
  `bal assets liabilities -M -H --layout=bare -b <today−6m> --forecast=<today>..<today+12m>`
  (`-O json`) → month-end balance per account per commodity, history + forecast. konto
  drops closed accounts and groups by commodity; the `Total:` rows are ignored. No math in
  konto.
- Horizon matches the Scheduled view's `+1y`.
- **No scenarios** (no `s` key, no `scenario` app state). **No drivers** — `ForecastDriver`,
  `Scenario`, `avg 6m` and the editable-average requirement are dropped.
- `v` valuation has no effect here: each commodity is shown in its own unit, no conversion.

### View

- **One braille chart per currency**, stacked vertically, all visible (few accounts
  expected). Each chart: one line per account holding that currency (an account with
  several currencies appears in each chart). 6 months history + 12 months forecast.
- **Colour = account**, fixed order green, blue, magenta, cyan, yellow, red (ANSI 2,4,5,6,3,1).
  Legend under each chart. A **dim vertical line** marks where the forecast begins.
- **Milestone table** below the charts: accounts as rows, grouped by currency; columns
  today, +3, +6, +12 months. **No totals** (neither table nor chart). Negative balances red.

### Amendment to ticket 13 — `once`

- Frequency dropdown gains **`once`** → writes `~ <date>  <description>` (verified 1.52.1:
  one occurrence). "Remaining payments" field hidden for `once`. Fits-form rule 1 extends to
  the bare-date expression.
- Past one-offs stay until deleted with `x`; status "none in the next 12 months", no
  special marker.

### New concept — closed account

- Marked by a `konto:closed:<date>` tag on the account's `account` directive (same reader/
  writer as `konto:source:`). hledger ignores it.
- **Closing is refused** unless the balance is zero **and** no `~` rule (active or ended)
  references the account ("still used by: <rule>").
- Closed accounts are visible **only in the account overview** (Dashboard) — excluded from
  Forecast, Entry, Import targets and the periodic-rule form's account picker.
- The close key/UI belongs to the Dashboard build.
