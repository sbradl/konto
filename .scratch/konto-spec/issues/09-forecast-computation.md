# 09 Forecast computation design

Type: grilling
Status: open
Blocked by: 02, 03

## Question

The Forecast view is a read-only projection — forecast transactions are never written to the
journal (hard product rule). Decide how it's computed:

- Use `hledger balance --forecast` (ticket 01 established what it emits) for the periodic
  part, or generate periodic occurrences in konto? Can the two feeds — exact `sched` rows
  and estimated `avg 6m` rows — be produced/distinguished cleanly?
- The trailing-average logic: 6-month window over what exactly (all non-periodic expense
  accounts? income too?), and how it projects forward.
- The three scenarios and precisely what each mutates:
  `base`, `lean (-15% spend)`, `3-month income gap` (income for the first 3 forecast months).
- Driver tagging so the drivers panel can show `sched` vs `avg 6m` per line, and the
  "average should be editable in place" requirement — does an in-place edit re-run the
  projection in memory only?
- Chart data shape: 6 months actual (green) joined to 12 months forecast (blue).

Feeds **Assemble docs/spec.md**.
