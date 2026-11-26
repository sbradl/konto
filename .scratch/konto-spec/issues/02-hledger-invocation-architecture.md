# 02 hledger invocation architecture

Type: grilling
Status: resolved
Blocked by: 01

## Question

Given ticket 01's findings on subcommands and output formats, decide how konto invokes
`hledger`:

- A single gateway/adapter package that owns every `exec` of `hledger`, or per-view calls?
- Synchronous reads vs `tea.Cmd`-wrapped async — and what the loading/pending UI contract is.
- Caching: is journal data loaded once per app run, on demand per view, or re-read on a
  trigger? What invalidates the cache (a write from Entry/Import/Scheduled, an external file
  change)? Is there a file watcher, or manual refresh only?
- Error surfacing: binary missing, command non-zero exit, unparseable output, journal file
  absent. What does the user see, and does the app degrade or refuse to start?
- Write coordination: manual entries, `.rules` edits, and "post next occurrence" all mutate
  files `hledger` then reads. How do writes and the read cache stay consistent?
- Where the version-floor check from ticket 01 happens (startup preflight?).
- **Direct journal-file access.** Ticket 01 established hledger has no command for verbatim
  `~` periodic-rule text or for `.latest.<file>` marker contents. Decide what konto reads/
  writes directly vs via hledger: does konto carry a partial journal parser, a full one, or
  just line-level extraction for the `~` blocks? Where does that live relative to the gateway?
- **Journal-check enumeration.** `check --strict` returns only the first failure (ticket 01).
  Decide the strategy for the `!` panel: run each check subcommand separately, parse stderr
  with per-check regexes, fix-rerun-repeat — or accept "first error only" for v1?

Feeds **Go package and module layout**, **CSV import subsystem design**,
**Test seams and fixture strategy**.

## Answer

Package names below are working names; **Go package and module layout** finalises them.

### Two data paths into the `Ledger` snapshot

1. **hledger gateway** (`internal/hledger`) — runs `hledger` subcommands, maps their JSON
   into konto domain types (ADR 0001). This is the main path.
2. **journal-text layer** (`internal/journal`) — a targeted line-oriented reader/writer for
   the few constructs hledger exposes no machine access to. **Not a journal parser** (the
   map already rules out a parser port). It handles only:
   - reading `~` periodic-rule blocks (the `~` line + indented postings + trailing
     comment/tags),
   - reading `account` directives and their tags (the `konto:source:` tag from ticket 03),
   - reading/writing `.latest.<file>` watermark files,
   - appending/editing those specific constructs plus new transactions and `.rules` blocks.
   It gets the include-set file list from `hledger files` and works per file. It never needs
   to understand transactions, amounts, or include resolution.
   (Commodity display style comes from `print -O json` `astyle`, not from parsing
   `commodity` directives — so the text layer does not need those.)

A **`Loader`** composes `*hledger.Client` + `*journal.Reader` and produces a `Ledger`
snapshot. The UI wraps `Loader.Load()` / `Loader.Refresh()` in a `tea.Cmd`.

### Gateway shape

- Plain **blocking Go**, no `bubbletea` import (grilled). `client.Balances(ctx) ([]Account,
  error)` etc. The UI wraps each call in a `tea.Cmd` goroutine that returns a result `Msg`.
- Two layers inside `internal/hledger`:
  - an unexported **command runner** `run(ctx context.Context, args ...string) ([]byte,
    error)` — the single `exec.Command` chokepoint, and the test seam for ticket 11 (fake
    it with recorded fixtures);
  - an exported **`Client`** with one typed method per operation (`Balance`, `Print`,
    `Register`, `ARegister`, `Prices`, `Check`, `ImportDryRun`, `Version`) that builds args,
    calls `run`, maps output → domain types.
- `run` takes a `context.Context`; the UI adapter cancels an in-flight refresh that a newer
  one supersedes.
- Role interfaces (what a given view needs) are declared at the consumer per Go idiom, not
  in `internal/hledger`.

### Loading / caching

- **Eager** (grilled): at startup the `Loader` builds the full `Ledger` snapshot; konto
  holds it. hledger calls are sub-100ms for a personal journal.
- **Refresh triggers**: an explicit `r` key; automatically after any konto journal write;
  automatically on return from `$EDITOR`. **No filesystem watcher in v1.**
- **Granularity**: any journal write → **full `Ledger` refresh** (one `tea.Cmd` re-running
  the gateway calls and rebuilding the snapshot). Slice-level refresh is a later
  optimisation. A `.rules` edit does **not** touch the journal (it affects future imports
  only) — it invalidates import-staging state, not the `Ledger`.
- **In-flight UI contract**: stale-while-revalidate. The held snapshot always renders; a
  refresh sets an in-flight flag shown as a small status-bar indicator; panels never blank.
  First load (no snapshot yet) shows a centred "loading journal…". A failed refresh keeps
  the stale snapshot and shows an error indicator. (Model-composition detail is ticket 05.)

### Error posture (grilled)

| Situation | Behaviour |
|---|---|
| `hledger` missing on `PATH`, or version `< 1.32` | **Refuse to start.** Startup preflight before the TUI: `exec.LookPath` → `hledger --version` → parse `hledger X.Y(.Z)` → compare to floor. Print the problem + minimum version to stderr, exit non-zero. Parsed version stored in `LedgerMeta`. |
| No journal file configured / file absent | **First-run screen** offering to create a minimal skeleton journal at a chosen path. (This is the handoff's unspecified empty state.) |
| Journal exists but `hledger` cannot parse it | Show the parse error as a top-severity `CheckIssue`, offer jump-to-`$EDITOR`; do not enter the normal views until it parses. |
| A runtime `hledger` call fails / returns unparseable output | Keep the last good `Ledger` on screen; error indicator in the status bar + detail in the `!` panel; never crash. |

### Journal-check enumeration (`!` panel)

hledger's `check` reports only the first failure per invocation and has no severities
(ticket 01). konto's approach:

- Run a fixed set of checks, **each as its own `hledger check <name>` invocation**; parse
  the first-failure message from stderr into a `CheckIssue` (regex the `file:line[:col]`
  prefix). Each check contributes 0 or 1 issue per run → the panel shows ≤ ~10 rows.
- **Fix-one → refresh → next-instance-appears** is the working loop. It matches how a person
  clears check failures and hledger cannot do better. Document the limitation in the spec.
- Which checks: basic (`parseable`, `autobalanced`, `assertions`) always; strict
  (`balanced`, `commodities`, `accounts`) configurable, default on; opt-in (`ordereddates`,
  `uniqueleafnames`, `tags`, `recentassertions`) configurable, default off. → feeds
  **Configuration and ANSI-16 theming**.
- konto's own checks (unclassified account from ticket 03, and any future ones) run in Go
  over the `Ledger` and **enumerate fully**.
- Severity assigned by check name (ticket 03: `CheckIssue.Severity`).

### `$EDITOR` integration

`tea.ExecProcess` (Bubble Tea v2) suspends the TUI, runs `$EDITOR +<line> <file>`
(`$VISUAL` → `$EDITOR` → a sane default), resumes, then fires a full refresh. Used by the
`!` panel's jump-to-location and the `e` edit key on the list views.

### Journal writes — mechanism only

This ticket fixes that **`internal/journal` owns all journal mutation** and it is
line-oriented (append / targeted edit), and that every write is followed by a full refresh.
*Which* file in an include chain a given construct is appended to, and the Entry-commit and
post-occurrence specifics, are **Periodic transaction subsystem design** and the Entry-view
fog.
