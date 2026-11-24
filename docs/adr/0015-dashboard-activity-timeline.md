# Dashboard Activity: upcoming over today over recent, no spend chart

The Dashboard showed Upcoming under Accounts in the narrow left column and Recent beside it, with the
monthly spend chart under Recent. Once rows carry the balance of their account after them, the left
column was too narrow for a description, and the two lists read as two halves of one timeline anyway.

The Dashboard now has one **Activity** panel across the full width, under Accounts and the stat cards:

- the periodic rules' occurrences for the next year, the furthest at the top, then a yellow
  `━━ today <date> ━━` line, then the journal's transactions, newest first: one timeline, newest at the top;
- each row is date, description, category, amount and the balance of its account after it, amount and
  balance green or red by sign. Recent balances walk back from the account's balance, upcoming ones
  forward from it;
- each month opens, above its latest row, with its signed net change, then its income (green) and expenses (red), booked and
  upcoming together, outside the column layout;
- it opens with the today line in its middle and keeps that until it is scrolled; `t` (today) puts it
  back there from either panel; a first or last row
  with more beyond it reads `… N more`;
- `Tab` / `Shift+Tab` move the focus between Accounts and Activity (ADR 0014's mark and status-bar
  position apply).

The **spend chart is dropped**, and with it the six months of spend history the loader read: the
snapshot asks hledger for the calendar month only, for the net income card.

Placing the today line by kind, not by date: a journal transaction dated after today still sits under it.

Amends ADR 0014's panel list. Reversible: view-local in `internal/ui/dashboard`, plus `Ledger.Today`.

Status: accepted.
