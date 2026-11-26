# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Read first

- **`docs/spec.md`** — the settled architectural foundation. Read it in full before writing
  code. Stack + versions, ports-and-adapters layering, package tree, hledger integration,
  domain model, Bubble Tea composition, config/theming, testing, and the suggested build
  order all live there.
- **`CONTEXT.md`** — the domain glossary (ubiquitous language). Use these terms.
- **`docs/adr/`** — the hard-to-reverse decisions (0001 rich domain model, 0002 flat
  account model, 0003 ports-and-adapters, 0004 root-mediator Bubble Tea model, 0005 fixed
  ANSI-16 palette, 0006 periodic rules never posted,
  0008 always categorised,
  0009 import: journal is the watermark, 0010 no manual-entry UI in v1, 0011 v1 keyboard and chrome,
  0012 context-owned models, Account aggregate, shared Changeset write model,
  0013 konto-owned year-partitioned ledger, only this and last year loaded,
  0014 Dashboard panel focus, 0015 Dashboard Activity timeline).
- **`docs/design_handoff_hledger_tui/README.md`** — the frozen UX spec (four views — Entry deferred in v1,
  keymaps, panels, tokens). Authoritative except the deviations listed in `docs/spec.md` §11.
  The bundled `.dc.html` is a browser mock, **not code to port** — consult it for the `HINTS`
  strings and layout proportions only (keymap = spec §6.4, ADR 0011), never literal coordinates.
- **`.scratch/konto-spec/`** — the wayfinder map and decision tickets (the "why" behind
  every choice in the spec), plus `research/` (hledger output, overlay compositing) and,
  later, `samples/` (bank CSV exports).

## What this is

`konto` — a terminal UI frontend for [hledger](https://hledger.org) (plain-text accounting),
single-user personal finance. Three views: dashboard, CSV import with rule-building,
scheduled/periodic transactions (manual entry deferred, ADR 0010). Keyboard-only: arrows move, `1`–`3` switch views, `Tab` moves the Dashboard's panel focus, keys are the first letter of their action.

## State

All three views are built and their feature files pass end to end: Dashboard, Import (open, categorise,
write, staging, Postbank format), Scheduled (list, detail, new/edit form, delete),
on top of the preflight, architecture tests, e2e harness and CI. No `@wip` or `@needs-decision` scenario is left in
`features/` (`grep -c @wip features/*.feature`). Per-feature design lives in `docs/spec.md` §4 and the tickets in
`.scratch/konto-spec/issues/`.

## Non-negotiables (full detail in docs/spec.md)

- **Charm v2 stack**, vanity import paths: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`,
  `charm.land/bubbles/v2`. Not the `github.com/charmbracelet/.../v2` paths. v1 is frozen.
- **`hledger` binary is a hard runtime dependency**, `>= 1.32`, checked at startup. konto
  shells out; it does **not** reimplement hledger's parsing, balance math, `check`,
  `--forecast`, or valuation. The one exception is `internal/journal`, a targeted
  line-reader/writer for the handful of constructs hledger exposes no machine access to
  (`~` rules, `account` directive tags, `.latest` files) — never a full parser.
- **Ports-and-adapters** (ADR 0003): dependencies point inward to `internal/domain`; use
  cases declare their own port interfaces; `internal/hledger` / `internal/journal` satisfy
  them structurally; `cmd/konto` wires. Views never import the adapters or each other.
- **Colours are ANSI-16 indices** (`lipgloss.Color("0")`–`"15"`), never hex.
- **All layout responsive to `tea.WindowSizeMsg`** — never hardcoded to the mock's 180×60.
- **An account is manual or import-fed, never both.** **CSV
  dedup shows skipped rows as a list.** **CSV import writes a `= <balance>` assertion.**

## Commands

Toolchain (go, golangci-lint, gotestsum, gremlins, govulncheck, gitleaks, trivy, opengrep, hledger) is pinned in `mise.toml`;
`mise install` provisions it. `mise tasks` lists every task.

- `mise run build` — `go build ./...`
- `mise run vet` — `go vet -tags integration,e2e ./...`
- `mise run lint` — `golangci-lint run`
- `mise run check` — build + vet + lint + cover:check (fast gate; archtest runs inside the tests)
- `mise run qa` — everything CI runs on a PR: tidy, check, security (govulncheck, gitleaks, trivy, opengrep), test:integration, test:e2e,
  mutation:diff (`.github/workflows/qa.yml` calls the same tasks)
- `mise run test:e2e` — Gherkin suite (`features/`, harness in `e2e/`); godog flags after `--`,
  e.g. `mise run test:e2e -- -godog.tags=@spec-4.5`.

Thresholds start maxed by user directive — loosen a threshold in `mise.toml` only with a
recorded reason, never silently.

## Testing

Verified against this repo's `mise.toml` / installed toolchain.

- **Full test suite** — `mise run test` (unit, `-race`, gotestsum) ·
  `mise run test:integration` (adds `-tags integration`, shells out to real `hledger`).
  Raw equivalent: `go test -race ./...`
- **Single test by name** — `go test -run 'TestName$' ./internal/pkg`
  (or `gotestsum -- -run 'TestName$' ./internal/pkg`). Go has no file-scoped runner: run a
  whole file's worth by `-run` over its test names, or pass the files explicitly:
  `go test ./internal/pkg/foo.go ./internal/pkg/foo_test.go`
- **Build / typecheck** — `mise run build` (`go build ./...`) · `mise run vet`
  (`go vet -tags integration,e2e ./...`). Single package: `go build ./internal/pkg`
- **Coverage for a single file** — profile the package, then filter the per-function report
  to that file:
  `go test -coverprofile=coverage.out ./internal/pkg && go tool cover -func=coverage.out | grep 'foo\.go'`
  Line-by-line: `go tool cover -html=coverage.out` (or `mise run cover:html`).
  Whole-module gate: `mise run cover:check` (fails under `THRESHOLD`, default **100**).
- **Mutation testing** — `mise run mutation internal/pkg` (one directory, recurses) ·
  `mise run mutation` (whole module) · `mise run mutation:diff` (only lines changed vs
  `main`). Gate: no mutant may survive (efficacy, in `.gremlins.yaml`; gremlins ignores the
  threshold flags). Screens (`internal/ui`) skip the +/- and minus-sign mutants; test helpers
  are not mutated. Raw equivalent: `GOFLAGS=-count=1 gremlins unleash --coverpkg=./... internal/pkg`
  (`-count=1`: a cached test run makes every mutant falsely "time out").

## Workflow

Trunk-based: commit and push straight to `main`. No feature branches, no pull requests.

Test-first. Skills in `.claude/skills/`:
- `tdd-loop` — drives `tdd-red` → `tdd-green` slice by slice, with a `craft-review`-fed
  `tdd-refactor` at each batch boundary.
- `craft-review` — five-lens diff review (DDD, code smells, Clean Code, Pragmatic
  Programmer, Philosophy of Software Design).
- `clean-review` — runs the static gate (`.golangci.yml` maps linters to Clean Code rules,
  `internal/archtest`), then reviews only what tools can't judge: Clean Code, DDD, Pragmatic.
  New Clean Code rules go into `.golangci.yml` when a linter can check them.

To advance a deferred per-feature ticket: `/mattpocock-skills:wayfinder work through the map, ticket NN`.
