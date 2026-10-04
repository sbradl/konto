# konto owns a year-partitioned ledger; only this year and last year are loaded

Every hledger call parses the whole journal (about 1.1 s at 20 000 transactions, and startup
makes ~23 calls side by side). hledger has no way to skip old transactions: `-b` still parses
everything. konto does not cache or checkpoint (that would mean reimplementing hledger's balance
math, or guarding a cache against stale money). It controls what hledger is given instead.

- **konto owns the files.** The ledger is `main.journal` (the path `--file` / `LEDGER_FILE` names)
  plus sibling `YYYY.journal` files. There is no existing-journal support and no migration; a new
  ledger starts empty. `main.journal` holds only directives (`account` with tags, `commodity`,
  payees) and `~` rules, never transactions and never an `include` of a year file.
- **One load rule, no mode.** konto calls hledger with `-f main.journal -f <last year>.journal
  -f <this year>.journal`, whichever of those exist. A ledger with no year files is just
  `main.journal`.
- **Writes route by nature.** A transaction goes to the file of its own year, created on first
  write. Directives and rules go to `main.journal`.
- **Carry-over keeps balances right.** The first import that crosses into a new year writes a
  closing transaction at the end of the old year's file and an opening transaction at the start
  of the new year's file (the pair `hledger close --migrate` writes, `equity:opening-balances`), both
  tagged as managed. The next crossing import recomputes and rewrites them from the loaded files,
  so rows for the previous year imported late stay valid when the window slides. Carries are
  computed through hledger on the changeset text via stdin.
- **Older rows are kept, not loaded.** Rows dated before last year are written to their own year
  files with the carry chain between them; those files are not loaded.

Consequences: reports, dashboard spend and checks see the window only (no way to look further
back in v1; the files stay on disk and consistent so a later feature can). `hledger files` lists
the loaded set, so `internal/journal` needs no include resolution. Check issues point into the
right year file.

Rejected: a disk cache keyed by a content hash (stale money on a wrong hit); konto-owned
balance checkpoints (hledger cannot start from a stored balance); slicing one big journal by date
in memory (needs konto's own opening balances); user-managed archives (konto owns the files).

Status: accepted.
