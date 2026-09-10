# 08 CSV import subsystem design

Type: grilling
Status: open
Blocked by: 02, 03

## Question

The Import view is konto's largest owned write-path. Decide its design:

- CSV parsing: konto-owned, or lean on `hledger import` / hledger's CSV rules engine? What
  does hledger's `import` give us for free (ticket 01) vs what konto must do itself?
- Rule generation: how konto builds an `if <pattern> / account2 <target> / comment <text>`
  block for a selected row, where it writes it (`.rules` file discovery/location), and how
  it computes the match-count stats (matches in this file / across the journal / existing
  rule count / rows still unmatched after saving).
- Dedup: reading and writing the `.latest.<file>` marker, showing skipped rows as a legible
  list, and the manual override (`d` mark genuine dup / `u` undo and re-import).
- Balance assertions: deriving `= <balance>` on the last posting from the CSV's
  running-balance column — column detection, sign handling, currency.
- The posting-preview: generating the exact journal text that will be appended, and which
  file / include-chain it goes to.

Feeds the "journal writes beyond CSV import" fog and **Assemble docs/spec.md**.
