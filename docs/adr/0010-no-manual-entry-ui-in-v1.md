# v1 has no manual-entry UI; the model keeps manual accounts

The design handoff has six views, the third being Entry: hand-entered transactions for
accounts never touched by CSV import (cash, informal lending), plus a classify-on-open
prompt asking whether a new asset/liability account is manual or import-fed.

v1 ships **three views** — Dashboard, Import, Scheduled, on keys `1`–`3` —
and no manual-account UI at all:

- no Entry tab, no `internal/ui/entry` view, no `internal/entry` use case; Entry's `CMDS` /
  `HINTS` / keymap rows are not transcribed;
- no classify-on-open prompt. konto only ever runs on a journal it created (first-run
  skeleton — no preexisting ledgers), and every asset/liability account is created by
  **first import**: an unknown IBAN offers "create account" (suggested, editable name,
  `uniqueleafnames`-checked) and writes the `account` directive with
  `konto:source:import`, `konto:iban:` and `konto:format:` into the main journal. There is
  no "attach to existing account" step;
- the **domain model is unchanged**: `Account.Source` (`Manual | Import`), the
  `konto:source:` tag, the manual-xor-import rule and the unclassified-account WARN check
  stay. A manual account can still be declared by hand in `$EDITOR`; import refuses it.

Reversible: the Entry view slots back in as a sixth view over the unchanged model. Its
commit target is already decided — the file of the account's latest transaction, else the
main journal, date-ordered insert (same rule as import).

Status: accepted.
