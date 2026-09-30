# CSV import: hledger converts, konto owns dedup and writes; the journal is the watermark

The design handoff's Import view lists files from a watched folder with status dots, dedups
via hledger's `.latest.<file>` watermark, and offers `d`/`u` manual duplicate overrides.

konto instead:

- uses hledger only to **convert** CSV to transactions (`print -f <csv> --rules-file …`),
  never `hledger import` — its `.latest` watermark is keyed by filename (bank downloads are
  renamed each time) and it appends to the main journal, while konto must insert in date
  order, gate on categorisation and pre-check assertions;
- **derives the watermark from the journal**: an import-fed account only holds imported
  transactions, so its latest transaction date D splits the CSV into skipped (< D), new
  (> D) and compared-as-multiset (= D). No state file, no manual dup override;
- picks files with a filepicker (no watched folder, no file list);
- matches rule patterns itself in Go **only** to refuse overlapping rules — hledger remains
  the authority that applies rules.

Reversible: the conversion step can be swapped for a konto-owned reader if hledger's rules
engine proves insufficient; the watermark and write path are unaffected.

Status: accepted.

Amendment (2026-09-30): transfers between two import-fed accounts would put a transaction
from one account's CSV into the other account, breaking its watermark (rows before the
transfer date skipped) and double-writing the other side's row. Each side therefore imports
its own leg against a single transfer clearing account; the legs net it to zero.
