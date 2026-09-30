# 14 Obtain sample bank CSV files

Type: task
Status: open

## Question

Not a decision — material the CSV import subsystem design (ticket 08) is blocked on.

The user provides real **Postbank** CSV export file(s), and any other bank formats konto
should support at v1. For each, capture in the ticket answer:

- delimiter, character encoding, line endings
- any preamble / metadata rows, multi-section structure, header row(s)
- column list and which columns give: booking date, value date, description / payee,
  purpose text, amount, sign convention (separate debit/credit columns vs one signed
  column), running balance, currency
- date format, decimal mark, thousands separator
- how konto could recognise "this file is a Postbank export" (filename pattern, header
  signature)
- whether the export is newest-first or oldest-first

Store the **real, unredacted** sample files under `.scratch/konto-spec/samples/` — that
directory is **gitignored**; the files are a format reference only and never leave the
machine. Ideally two exports: one long period (3–6 months, in/out, card, direct debit,
transfer, PayPal, a pending booking, a repeated shop) and a second downloaded a few days
later that overlaps it (dedup, row-order stability, pending→booked, filename change).

**Test fixtures are never copies of the real files.** The build creates synthetic,
anonymised CSVs under `testdata/` that reproduce the format exactly (encoding, delimiter,
preamble, columns, quirks) with invented names, IBANs, purposes and amounts — running
balances kept internally consistent so assertions and opening-balance derivation are
testable. The answer records the format
findings; ticket 08 designs the abstraction from them.
