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

Store the sample files under `.scratch/konto-spec/samples/` (redact balances/account
numbers if the user wants — structure is what matters). The answer records the format
findings; ticket 08 designs the abstraction from them.
