# Periodic rules are forecast-only; no post-occurrence

The design handoff's Scheduled view lets Enter "post the next occurrence into the real
journal now", plus a `p` pause state. konto drops both. hledger `~` periodic rules only feed
projections — Forecast, Dashboard Upcoming, the Scheduled view's next-occurrences and annual
impact. konto never writes an occurrence into the journal; real transactions arrive only via
CSV import or the Entry view.

Posting occurrences would double-count against CSV import for any rule touching an
import-fed account (most of them — rent, salary, subscriptions go through the bank), and
the user wants the Scheduled view purely as a forecasting tool. Pause is dropped as
redundant: a rule is ended (remaining-payment count), deleted, or replaced.

Consequence: `internal/journal` has no post-occurrence or pause-toggle write; its periodic
writes are append/replace/delete of a `~` block only.

Status: accepted.
