# Rich konto domain model behind a mapping layer

konto shells out to `hledger` and could consume its JSON output directly, but that JSON is a
verbose, version-drifting mirror of hledger's internal Haskell types (positional arrays for
`balance`, deeply nested amount styling) and carries none of konto's own concepts. We decided
that a gateway layer decodes hledger's output — tolerantly, with any version shims in one
place — into konto-owned domain types (see `CONTEXT.md`), and the rest of the app only ever
sees those types.

The cost is a mapping layer to write and maintain. The benefit: views survive hledger
upgrades, positional JSON never leaks into UI code, and konto-only concepts (account source,
"+ N cur", uncategorized markers, report row kinds, import staging, forecast drivers) have a
natural home. Reversing this later would mean rewriting every view against hledger's schema.

Status: accepted.
