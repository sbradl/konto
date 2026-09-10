# 08 CSV import subsystem design

Type: grilling
Status: open
Blocked by: 02, 03, 14

## Question

The Import view is konto's largest owned write-path. Decide its design:

- CSV parsing: konto-owned, or lean on `hledger import` / hledger's CSV rules engine? What
  does hledger's `import` give us for free (ticket 01) vs what konto must do itself?
  Note: konto needs its own raw-CSV read regardless (ticket 01 — `import --dry-run` won't
  enumerate skipped rows, and the Import view's row table shows parsed rows + match state).
- **Multi-format support.** Primary target is **Postbank** CSV exports (samples via ticket
  14); the subsystem must be extensible to other banks. Decide the abstraction:
  - A **bank profile / format** concept, one per bank, carrying: raw-parse settings
    (delimiter — German banks use `;`; character encoding — often ISO-8859-1 / Windows-1252,
    not UTF-8; decimal comma; date format e.g. `DD.MM.YYYY`; header/preamble rows to skip;
    Postbank exports may have a metadata preamble and/or multiple CSV sections), the
    running-balance column, and the hledger `.rules` file template.
  - Single source of truth for column layout: does konto read it from the hledger `.rules`
    `fields`/`skip`/`date-format` directives, or from its own profile config, or both?
  - hledger's `.rules` engine already adapts arbitrary CSV → hledger fields — how much
    format handling is delegated to it vs owned by konto's reader for the preview.
  - Where profiles live (built-in Go + user-supplied config?), how a file is matched to a
    profile (filename glob? header signature? explicit per-import-source config?).
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
