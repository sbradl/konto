# Dashboard panel focus: Tab between Accounts, Upcoming and Recent

ADR 0011 gave each view at most one cursor panel and no panel focus: the Dashboard's Recent and
Upcoming panels showed `… N more` when they overflowed, with no way to see the rest. Once Upcoming
moved under Accounts and Recent grew to most of the right column, both hold more than one screen of
rows that are worth reading in place.

The Dashboard now has a **panel focus**:

- `Tab` / `Shift+Tab` move it through Accounts → Upcoming → Recent and wrap, skipping panels the
  terminal has no room for; when the focused panel stops fitting the focus falls back to Accounts;
- the focused panel's title carries a ` ◂` mark (`╭─ RECENT ◂`);
- the cursor keys drive the focused panel: the Accounts cursor as before, or scroll Upcoming and
  Recent a row or page at a time until their last entry shows. Their rows stay unselectable;
- `c` / `o` still act on the selected account wherever the focus is;
- the status bar's `n/total` is the focused panel's: the Accounts cursor row, or the first row a
  scrolled panel shows.

Only the Dashboard has panel focus. Every other view keeps ADR 0011's single cursor panel.

Amends ADR 0011 ("no panel focus") for the Dashboard only. Reversible: focus is view-local state in
`internal/ui/dashboard`; the root and the use cases are unchanged.

Status: accepted; its panels amended by ADR 0015 (Upcoming and Recent are one Activity panel).
