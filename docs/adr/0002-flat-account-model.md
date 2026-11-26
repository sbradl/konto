# Flat account model; Accounts panel is a flat list

hledger accounts form a tree, and its `balance` output includes parent/aggregation rows.
konto models accounts as a flat list of postable accounts only — no parent entities — and
the Dashboard's Accounts panel renders a flat, non-indented list of full account names
(name + balance + source glyph) rather than the indented tree with header and subtotal rows
shown in the design handoff (`docs/design_handoff_hledger_tui/README.md`).

This is a deliberate deviation from the otherwise-frozen handoff UX, chosen for model and
view simplicity in a single-user tool.

Status: accepted.
