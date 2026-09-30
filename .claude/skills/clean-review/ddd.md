# Domain-Driven Design lens — judgement rules

Evans / Vernon, applied to konto's settled model. Package layering, allowed imports, the clock seam and colour rules are enforced by `internal/archtest`; this file holds what a tool cannot see. Sources of truth: `CONTEXT.md` (language), ADR 0012 (where types live, aggregates, `Changeset`), ADRs 0001–0011. Each rule: _what to look for_ → _fix_.

## Language

- **Drift** — an identifier, message, UI string or test name using a word `CONTEXT.md` marks _Avoid_, or a synonym for a glossary term (`Draft` for staged import, `Instance` for occurrence, `Cursor` for watermark). → the glossary term.
- **Missing term** — the diff introduces a domain concept the glossary lacks. → flag the gap; the glossary is updated before the code spreads the new word.
- **UI vocabulary** — UI copy saying "commodity" or "account" for `expenses:*`/`income:*`. → "currency", "category" (spec §10).

## Contexts

- **Model placement** — a type only one context uses added to `internal/domain`, or a shared-kernel type redefined in a use case (ADR 0012). → context-owned types live in their use-case package; shared ones in `domain` once.
- **Anti-corruption leak** — hledger JSON shapes, field names or positional arrays past `internal/hledger`; journal text structure past `internal/journal`. → translate at the adapter into domain types.
- **Port shape** — a use case's port named after the adapter (`HledgerClient`) or wider than the use case needs. → consumer-side, named for the need, minimal (ADR 0003).
- **ADR contradiction** — the diff contradicts an ADR. → say which and why it is worth reopening; the ADR stands until then.

## Aggregates and values

- **Anemic model** — a rule about an `Account`, `StagedImport` or `PeriodicRule` decided in a view, service or adapter instead of on the aggregate. → move the rule onto the aggregate.
- **Invariant bypass** — code that can produce an aggregate state its invariants forbid (a closed clearing account, a writable import with `?` rows, overlapping rules). → only aggregate methods change that state.
- **Invalid construction** — a constructor or literal that can build an invalid value (`Amount` with mismatched scale, empty account name). → validate in a constructor; unexported fields where it matters.
- **Mutable value object** — a value type with pointer receivers that mutate, or shared slices callers can modify. → value receivers, copies out.
- **Primitive obsession** — a bare `string`/`int`/`time.Time` for a glossary concept (account name, rule id, report period, watermark date). → a small named type.
- **Write path** — a journal write not expressed as a `domain.Changeset` through the use case's `journalWriter` port, or not followed by validate → apply → refresh (ADR 0012). → produce a `Changeset`.
- **Cross-view effect** — a view changing another view's state or calling the `Loader` instead of emitting `journalChangedMsg` (ADR 0004, 0012). → a message to the root.
- **Snapshot mutation** — anything mutating the `*Ledger` instead of replacing it. → a new snapshot via refresh.

## Product rules (spec §10)

Flag any hunk that can break: manual xor import; forecast never written; periodic rules never post; every transaction categorised, a transfer or an opening balance; import writes the `=` assertion; skipped rows as a list; every hledger check passes after a write; closed accounts only in the account list.
