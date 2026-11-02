# Every transaction is categorised; no catch-all account

The design handoff marks uncategorised transactions with a yellow `?` on the Dashboard and
offers a `c` categorise key, implying transactions may sit in a catch-all account
(hledger's CSV default `expenses:unknown` / `income:unknown`) until fixed later.

konto forbids that state. There is no catch-all account: CSV import writes only once every
row has a category (an existing `expenses:`/`income:` account or a newly created one plus
its rule) — rows cannot be skipped — and manual entry requires a category. Consequently the
domain has no `Uncategorized` flag, and the Dashboard has no `?` marker or `c` key.

Rationale: the user wants the journal correct at all times rather than carrying a backlog
of placeholder postings. Reversing this later would require a catch-all concept, a marker
and a categorise flow, but no data migration.

Status: accepted.

Amendment (2026-09-30): "categorised" means the counterpart is a category **or** another
account. A **transfer** (to savings, card payoff) picks an asset/liability account in the
category picker; an **opening balance** posts against `equity:opening-balances`. Forcing
either into a category would distort net income.

Amendment (2026-11-02): a **fallback rule** holds the rows no other rule matches (PayPal in general), so that a
large import can be written before every payee has its own rule. It names a category the user chose and a pattern
they typed, so this is still no catch-all account: no row lands somewhere konto picked. A pattern of `.` makes one
cover every remaining row that has a payee; that is the user's choice to make.
