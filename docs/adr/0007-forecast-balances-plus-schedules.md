# Forecast is current balances plus scheduled transactions, per account

The design handoff's Forecast view projects a single net-worth line from periodic
transactions plus six-month trailing averages, under switchable scenarios (`base`, `lean
−15%`, `3-month income gap`), with a projection table and a drivers panel whose averages are
editable in place.

konto drops the estimation entirely. The forecast is each open asset/liability account's
current balance plus the occurrences of its periodic rules — nothing is averaged or guessed;
the user keeps the rules current. Accordingly there are no scenarios, no drivers panel and no
projection table. The view instead shows one braille chart per currency (one line per
account, a vertical marker where the forecast begins) over a milestone table (today, +3, +6,
+12 months per account).

All figures come from a single `hledger bal assets liabilities -M -H --forecast=…` call;
konto performs no projection math and no currency conversion in this view.

Status: accepted.

Amendment (2026-09-30): the projection for each import-fed account starts at its watermark
(latest transaction date), not today — the journal balance is only as fresh as the last
import, and the rules already describe what happened since. This needs one `bal` call per
account instead of one overall.
