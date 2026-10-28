# Context-owned models, Account aggregate, shared Changeset write model

ADR 0003 sets the rings. It leaves open where konto-only types live, where account
close/reopen and the check run are orchestrated, and how the several write paths (import,
periodic rules, close/reopen) reach the journal. We decided:

- **Context-owned models live in their use-case package.** Types that only one bounded
  context uses stay there: `StagedImport`, `CsvRow`, `ImportRule`, `Watermark` in
  `internal/csvimport`; `RuleSpec` (the periodic-rule form) in `internal/periodic`.
  `internal/domain` is the **shared kernel** only: `Amount`, `MultiAmount`, `Commodity`,
  `Account`, `Transaction`, `Posting`, `PeriodicRule`, `SourcePos`, `CheckIssue`,
  `ReportTable`, `Changeset`.
- **`Account` is an aggregate with behaviour.** `Close(balance, referencingRules)`,
  `Reopen()` and `CanBeImportTarget()` enforce the account invariants (source immutable,
  clearing account never closed or imported into, close only at zero balance with no
  referencing `~` rule, closed → no import) and return the `Changeset` to write. A new
  **`internal/accounts`** use case orchestrates close/reopen.
- **One write model.** `domain.Changeset` is a list of targeted `Edit`s (insert transaction
  at date position, set/remove account tag, declare account/payee, append/replace/delete a
  `~` block, append a `categories.rules` block or matcher line). Every writing use case
  declares its own `journalWriter{ Apply(domain.Changeset) error }` port; `internal/journal`
  implements it once (per-file temp + rename). Every write runs validate → apply → refresh.
- **A new `internal/checks` use case** owns the check run: one `hledger check <name>` per
  check, severity by name, konto's own checks (unclassified account). `CheckRun` leaves the
  `Ledger` snapshot; the root holds the latest one.
- **"Today" is injected** as a `today func() domain.Date` constructor argument on every
  service that needs it; `cmd/konto` passes a wrapper over `time.Now` in local time.
- **Writes signal the root with `journalChangedMsg`.** The writing view emits it; the root
  shows the confirmation, refreshes the `Ledger` and reruns checks — the same path as `r`
  and return from `$EDITOR`. Views and write use cases never hold the `Loader`.

Alternatives rejected: all entities in `domain` (import-only concepts bloat the shared
kernel and couple contexts); per-construct writer methods on the journal adapter (a
multi-edit import is not one unit, three bespoke write paths); a write use case returning a
fresh `Ledger` (couples every writer to the `Loader`); checks inside the `Loader` (checks
only refresh with a full reload, and the snapshot mixes journal data with journal health).

Status: accepted.

Amended (as built): there is no `journalChangedMsg` and no view emits one — views run no commands
(ADR 0004, amended). A view's action makes the root plan the `Changeset` and ask y/n; the root
verifies and applies it, and on `writtenMsg` shows the confirmation and reloads through the same
`reload()` as `r` and the return from `$EDITOR`. Views and write use cases still never hold the
`Loader`.
