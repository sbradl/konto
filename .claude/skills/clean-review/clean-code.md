# Clean Code lens — judgement rules

Martin, _Clean Code_ 2nd ed. Sizes, complexity, nesting, argument counts, flag arguments, magic numbers, duplicated blocks, name length, comment punctuation, TODOs, commented-out code, unwrapped errors, `nil, nil`, globals, `init`, line length and formatting are enforced by `.golangci.yml`; this file holds what a tool cannot see. Each rule: _what to look for_ → _fix_.

## Names

- **Intention-revealing** — the name forces the reader into the body to learn what it is for. → name the purpose.
- **Disinformation** — `list` that is a map, `total` that is a subtotal, two names a letter apart. → name what it is.
- **Noise words** — `Data`, `Info`, `Manager`, `Helper`, `Util`, `Processor` making a distinction without a difference. → name the actual responsibility.
- **One word per concept** — `load`/`fetch`/`read`/`get` for the same act across packages. → pick one; the glossary's word if it has one.
- **Problem-domain names** — code invents a term `CONTEXT.md` already names, or uses an _Avoid_ word. → use the glossary term (the DDD lens owns drift; flag here only unclear names).
- **Pun** — one word for two different operations (`add` as sum here, as append there). → a different name for each.

## Functions

- **One thing** — you can extract a function whose name is not a restatement of the original. → extract until you cannot.
- **One level of abstraction** — `strings.Split` next to `importer.Stage(...)` in one body. → push the detail down into a named helper.
- **Step-down** — a caller sits below its callees, or the file reads bottom-up. → callers above callees, one level down at a time.
- **Command-query separation** — a function both changes state and answers a question (`Close()` returning whether it could). → split, or return an error only.
- **Hidden side effect / temporal coupling** — a call that must happen before another with nothing enforcing the order; a "check" that also mutates. → make the dependency visible: pass the result of the first into the second.
- **Output argument** — a parameter mutated to return a result. → return it.
- **Argument object** — three or four parameters that always travel together. → give them a type named for the concept.

## Comments

- **Redundant** — says what the next line says. → delete.
- **Misleading or stale** — disagrees with the code. → fix or delete.
- **Should be a name** — explains a variable or expression. → rename or extract, then delete.
- **Good comment missing** — a non-obvious _why_ (a gotcha, a hledger quirk, a deliberate deviation) with no comment. → one line of intent.

## Objects and data structures

- **Hybrid** — a type that both exposes its fields for callers to decide and carries behaviour on them. → an object (behaviour, hidden data) or a data structure (exposed data, no behaviour).
- **Law of Demeter / train wreck** — `a.B().C().D()`, or reaching into a returned value's fields to decide. → tell the first object what you want.
- **Feature envy** — a function using another type's fields more than its own. → move it onto that type.

## Errors

- **Message quality** — an error message that does not say what failed and with which input. → name the operation and the value.
- **Errors shaped for the caller** — callers must string-match or unwrap to react. → a sentinel or typed error the caller can `errors.Is`/`As`.
- **Special case** — callers repeat the same `if err` / empty check. → a special-case value that behaves correctly.

## Boundaries

- **Third-party type spreading** — a Charm, godog or `exec` type passed beyond the package that owns the boundary. → wrap it in a type you own.
- **Unpinned third-party behaviour** — code relies on hledger or library behaviour no test pins. → a learning test (an `integration` test for hledger).

## Tests

- **One concept per test** — a test checking several behaviours, or a table mixing success and failure paths. → split by concept.
- **Readable test** — setup noise buries the check; no build-operate-check shape. → extract setup into helpers named in domain terms.
- **F.I.R.S.T.** — depends on order, wall-clock time, the network, or shared temp state; checks by eye (logs) instead of assertions. → isolate and assert.
- **Dual standard** — test code sloppier in names or structure than production. → same standard.

## Types, systems, emergence

- **Single responsibility** — a type or package with two reasons to change (for example rendering and hledger mapping). → split along the reasons.
- **Cohesion** — methods using disjoint subsets of the fields. → split the type.
- **Construction separated from use** — something other than `cmd/konto` wires concretes. → move the wiring to the composition root.
- **Encapsulate conditionals** — a compound boolean inline (`a && !b || c`). → a named predicate; prefer positive names over negated ones.
- **Boundary conditions** — `+1`/`-1` scattered around. → one named variable for the boundary.
- **Configurable data high** — a constant buried deep that belongs at the top of the package or in the caller.
- **Four rules of simple design** — tests pass, no duplication, expresses intent, fewest elements. Flag a type, interface or indirection that earns nothing yet.
- **Concurrency** — a goroutine sharing mutable data without a stated owner; a `tea.Cmd` result applied without its generation check (spec §6.3). → one owner, message passing.
