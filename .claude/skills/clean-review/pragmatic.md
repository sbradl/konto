# Pragmatic Programmer lens — judgement rules

Hunt & Thomas, 20th-anniversary ed. Copy-paste blocks, ignored errors, `TODO`s, globals and unchecked type assertions are enforced by `.golangci.yml`; the maxed coverage and mutation gates enforce test existence. This file holds what a tool cannot see. Each rule: _what to look for_ → _fix_.

## DRY — knowledge, not text

- **Duplicated knowledge** — one fact in two forms that must change together: a rule in Go and in a comment, a format in the harness and in konto, a threshold in two packages, spec text restated in code comments. → one authoritative source; derive or point to it.
- **Parallel structures** — adding a case means editing two lists in step (a key table and a help table, a switch and a map). → one table both read.

## Orthogonality

- **Shotgun change** — the diff edits several unrelated packages for one behaviour. → the seam is in the wrong place; hide the varying part behind one.
- **Tell, don't ask** — a caller pulls state out of an object to decide what the object should do. → a method on the object.
- **Detail in logic** — a path, command-line flag, hledger argument or UI string hard-wired deep in a decision. → a named constant or parameter at the edge.

## Contracts and crashing early

- **Unstated precondition** — a function that misbehaves on empty, zero, negative or unsorted input without saying so. → reject at the top with an error, or document the contract.
- **Weak postcondition** — a function can return a partly-built result alongside a nil error. → validate before returning.
- **Dead program over crippled one** — code that notices an impossible state and carries on (default branch returning a zero value, a logged-and-continued error). → return the error or fail loudly where it is detected.
- **Balanced resources** — a file, lock, process or pty opened with no release on every path. → release in the same function (`defer`), or hand ownership to one clearly named owner.

## Deliberate programming

- **Coincidence** — code relying on map order, goroutine timing, an unguaranteed default of hledger or a library, or a sleep. → rely on documented behaviour; synchronise explicitly.
- **Unproven assumption** — the diff blames or works around hledger/Charm behaviour with no test or probe showing it. → a learning test first.
- **Borrowed code not understood** — pasted or generated code the change cannot explain line by line. → understand it or remove it.

## Reversibility

- **One-way door** — a vendor, file format or journal layout assumption spread beyond its adapter. → keep it behind the adapter so the choice stays cheap to undo.
- **Speculative generality** — an interface, option or hook with one implementation and no second case in sight (YAGNI). → remove until the second case arrives.

## Broken windows and tracer bullets

- **Workaround next to the fix** — the change routes around a bad structure it touches instead of repairing it. → refactor while you are there, or name the debt in the report.
- **Big-bang change** — a slice that cannot be exercised end to end until more lands. → the thinnest path through all layers first.

## Tests as design feedback

- **Hard to test** — a test needing heavy setup, sleeps or real I/O to reach logic. → the design lacks a seam; inject it.
- **Behaviour vs implementation** — a test asserting call order or private structure rather than outcome. → assert on observable results.
- **Bug fix without the test that catches it** — find bugs once.
