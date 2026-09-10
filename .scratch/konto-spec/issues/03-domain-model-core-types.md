# 03 konto domain model and core types

Type: grilling
Status: open
Blocked by: 01

## Question

Define konto's domain model (writes `CONTEXT.md`, and an ADR if the read/write-model split
is a real hard-to-reverse trade-off):

- Where the boundary sits: thin DTOs that mirror `hledger` JSON output 1:1, or a richer
  konto-owned model that the gateway maps into? Different answers per concern (e.g. thin for
  reports, rich for the import-staging state that has no hledger equivalent)?
- The core nouns and their relationships: Journal, Account (+ the manual-vs-import-fed
  distinction — a hard product rule), Transaction, Posting, Amount, Commodity,
  PriceDirective, PeriodicTransaction, ImportRule, CsvRow, Check/CheckResult.
- How valuation mode (`native` | `market`) is represented — a mode flag passed to the
  gateway, or two pre-computed views held in memory?
- How multi-commodity amounts are modelled so "+ 2 cur" and market collapse both fall out
  naturally.
- Which concepts are konto's own with no hledger counterpart (import staging,
  `.latest.<file>` markers, rule match-stats, forecast drivers) and therefore fully owned here.

Feeds **Go package and module layout**, **Bubble Tea model composition**, **CSV import
subsystem design**, **Forecast computation design**, **Test seams and fixture strategy**.
