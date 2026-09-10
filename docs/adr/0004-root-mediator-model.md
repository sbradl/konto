# Root-mediator Bubble Tea model

Bubble Tea runs exactly one `Model`. konto's is a root model in `internal/ui` that owns all
app-global state (`valuationMode`, `period`, `scenario`, terminal size, the read-only
`*domain.Ledger` snapshot), the chrome, the overlay state, and the six view sub-models —
which are all constructed at startup and kept alive for the whole run.

State changes propagate as **messages, not shared pointers**: a view acts, the journal or
global state changes, and the root broadcasts to every view. Views never hold references to
each other. Input messages go to the active view only; data messages are broadcast; view-
specific async results are routed by the root to the owning view, with a per-view generation
counter dropping stale results.

The alternative — a shared `*AppState` pointer held by root and all views — was rejected: it
breaks Bubble Tea's value-model contract, makes each `Update` impure, and makes views
untestable in isolation. Letting views reference each other for cross-cutting refreshes was
rejected for the coupling ADR 0003 forbids. The cost is message plumbing in the root's
`Update`.

Status: accepted.
