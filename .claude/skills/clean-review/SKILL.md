---
name: clean-review
description: Review a diff for the Clean Code, Domain-Driven Design and Pragmatic Programmer rules that no linter can judge — after the static gate has run.
disable-model-invocation: true
---

Tooling judges what it can; this review judges the rest. `.golangci.yml` (each linter annotated with the Clean Code rule it enforces) and `internal/archtest` (layering, ADRs 0003/0004/0005/0012) are the **static gate**. The lens files list only **judgement rules** — the ones a tool cannot decide. A finding the gate already reports is the gate's; the review never restates it.

## 1. Pin the diff

The fixed point is what the user named (SHA, branch, tag, `HEAD~3`); default is everything not on `main` plus uncommitted work: `git diff $(git merge-base HEAD main)` (covers commits and the working tree). Capture the diff command, the changed `.go` files, and `git log <fixed-point>..HEAD --oneline`.

Done when the ref resolves and the diff has at least one changed Go file. An empty diff ends the review here with that one line.

## 2. Run the static gate

Run `mise run lint` and `go test -count=1 ./internal/archtest/`. Keep every finding that lands on a changed file.

Done when both commands have run and their findings on changed files are listed. These go to the report verbatim under **Static gate**; fixing them comes before judging the rest, so say so if the list is non-empty.

## 3. Gather shared context

Read `CONTEXT.md`, the ADRs in `docs/adr/` that touch the changed packages, and the `docs/spec.md` sections for those packages. This context goes to every lens: a documented decision overrides a lens heuristic.

## 4. Run three lenses in parallel

One message, three `Agent` calls (`general-purpose`), so no lens's clean bill masks another's finding. Resolve `<skill-dir>` to this folder's absolute path. Every prompt carries the diff command, the commit list, the step 3 context, the static-gate findings from step 2, and this brief:

> Read `<skill-dir>/<lens>.md`. Apply **every** rule to **every** changed hunk; a rule with no hit needs no mention. Findings the static gate already reported are out of scope. Each finding: rule name, `file:line`, the quoted hunk, the fix in one line, and a confidence of *sure* or *judgement*. Under 500 words.

Lenses: `clean-code`, `ddd`, `pragmatic`.

Done when all three reports are back.

## 5. Report

Sections in this order: **Static gate**, **Clean Code**, **Domain-Driven Design**, **Pragmatic Programmer**. Keep each lens's findings as it returned them. A hunk flagged by two lenses stays under both; the overlap is signal.

When two lenses give opposite fixes for one hunk, keep both and mark the loser "overridden by <lens>" with precedence **DDD → Pragmatic → Clean Code**.

End with one line per section: finding count and the worst finding.
