# features — e2e acceptance tests (Gherkin)

User's-eye tests of the compiled `konto` binary. One `.feature` per spec area; each cites its
source in `@spec-N.N` / `@adr-NNNN` tags so every requirement is traceable.

| file | area |
|---|---|
| `startup` | preflight, journal path, first-run, parse error, too-small terminal, lock |
| `chrome` | tab/status bar, global keys, `?` popup, refresh, quit, responsive layout, ANSI-16 theme |
| `checks` | `● clean` indicator, `!` panel, every check + severity, unclassified WARN, `$EDITOR` jump |
| `dashboard` | stat cards, Accounts / Activity (upcoming over today over recent), dropped panels |
| `accounts` | close (`c`) / reopen (`o`) |
| `import-flow` | picker, IBAN detection, row table, skipped list, watermark dedup, bad input |
| `import-categorise` | popup, once / new rule / add to rule, overlap refusal, category picker, transfers |
| `import-write` | `w` gating, written text, assertion block, opening balance, new account, after-write |
| `import-staging` | staged work survives view switches, `q` guard, discard |
| `import-format` | raw Postbank bytes: BOM, preamble, trailer, German numbers/dates |
| `scheduled` | list, panels, form (`n`), edit (`e`), delete (`d`), never-posts |
| `forecast` | chart per currency, milestones, watermark start, never-written |
| `reports` | period cycling, balance sheet Δ, income statement, boundaries, scrolling |
| `ledger-window` | only this and last year loaded, carried-in balances, the window follows today (ADR 0013) |
| `ledger-carry` | year-end close/open on the first import of a new year, older rows into their year files (ADR 0013) |
| `invariants` | cross-cutting product rules (checks after every write, manual xor import, …) |

## Tags

`@spec-N.N` `@adr-NNNN` traceability · `@proposed` spec is silent, scenario encodes the obvious
reading — confirm or change · `@needs-decision` two readings are plausible, blocks the test (none open) ·
`@wip` not implemented yet · `@integration` needs real hledger · `@slow`

## Harness — `e2e/`

- `mise run test:e2e` runs godog (`//go:build e2e`) over these features; godog flags after `--`
  (`mise run test:e2e -- -godog.tags=@spec-4.5 -godog.format=progress`). Default filter
  `~@wip && ~@needs-decision`, strict: a non-`@wip` scenario with an undefined step fails.
- `@wip` marks a scenario not built yet (none left). Building a slice = remove `@wip`
  from its scenarios, add any view-specific steps, make them pass.
- Drives the **compiled binary** (built lazily from `cmd/konto`) in a PTY (default 120×40),
  rendered through `charmbracelet/x/vt`; asserts on screen text, cell colour (ANSI index), exit
  code, stderr, and the files on disk. The harness imports nothing of konto's (archtest).
- One sandbox per scenario: temp dir with its own `HOME`, XDG dirs, download dir, journal
  (`LEDGER_FILE`), `KONTO_TODAY` (default 2026-09-30), `EDITOR` stub.
- **Idle signal, no sleeps.** The harness sets `KONTO_E2E=1`; konto (`cmd/konto/idle.go`) then sets the
  window title, after each frame, to `konto idle N` or `konto busy N`: busy while a command it started (load,
  write, plan, report, forecast) is in flight, N the keys and resizes taken in. After each key the harness
  waits for N to count it and for idle; with `hledger is slowed` it only waits for the key to be taken in,
  so the scenario can watch the work in flight. "Unchanged" and other absence checks wait for idle first.
- **`hledger` shim** (`e2e/hledgershim`) first on `PATH`: records every call, fakes the version,
  slows, fails or returns garbage per scenario, else execs the real pinned hledger.
- **Postbank CSV generator** renders the table steps into a full export (BOM, preamble, header,
  rows newest first, `Kontostand` trailer). No real bank data anywhere.
- `the journal contains` compares whitespace-normalised consecutive lines; `the journal is
  unchanged` is a byte comparison against the state at `I start konto`; `the screen is
  unchanged` compares against the screen before the first action of the When block.
- The shim's config is one type, `e2e/shimconf`, shared by the shim and the harness; the shim
  refuses a field it does not know.
- Failures dump screen, stderr and hledger calls to `$KONTO_E2E_ARTIFACTS` (CI uploads them);
  `KONTO_E2E_KEEP=1` keeps the sandboxes.
- Harness self-tests (`e2e/harness_test.go`) pin the pty, screen, keys, shim, CSV generator and
  matchers, so a green run means konto behaved.

## Fixtures — `testdata/journals/` (hand-written, verified against hledger 1.52.1)

All pass all 11 `hledger check`s. Numbers in the features come from these.

- `minimal` — skeleton only (clearing account, tags).
- `household` — 54 transactions Jan–Sep 2026. checking 13.240,00 · savings 13.500,00 ·
  creditcard -140,00 · spare 0,00 (open) · old-account 0,00 (closed 2026-03-01) · transfers 0,00.
  Net worth 26.600,00 EUR. September: income 3.000,00, expenses 1.185,00, net 1.815,00.
  Watermarks: checking/savings 2026-09-26, creditcard 2026-09-14.
- `household-scheduled` — `household` + 5 `~` rules (rent, salary, savings, yearly insurance,
  streaming-with-comment).
- `multicommodity` — EUR/PLN/CZK wallet (manual) + one import account, 2 rules.
- `categories.rules` — 5 rules (ids 1–5) matching `household` payees.

## Step index — `e2e/`

Harness, no steps: `world_test.go` (sandbox, start, press, waits), `session_test.go` (pty, screen,
idle signal), `asserts_test.go` (`eventually`, file asserts), `journal_test.go` (block match, hledger
checks), `keys_test.go`, `postbank_test.go` (CSV generator), `harness_test.go` (self-tests).

| steps file | covers |
|---|---|
| `steps_test.go` | the generic vocabulary below: journals, shim, editor stub, start/press/type/resize, screen, process, files |
| `journal_paths_test.go` · `firstrun_steps_test.go` · `instance_steps_test.go` | startup: path resolution, first run and parse error, the lock |
| `chrome_steps_test.go` · `chrome_extra_steps_test.go` · `popup_steps_test.go` | tab/status bar, refresh, theme, layout, `?` popup |
| `checks_steps_test.go` | checks indicator and `!` panel, check fixtures, `$EDITOR` jump |
| `stat_card_steps_test.go` · `accounts_panel_steps_test.go` · `recent_steps_test.go` · `panels_steps_test.go` | dashboard panels |
| `account_change_steps_test.go` | close/reopen accounts, prompts, refusals, `account` directives |
| `import_steps_test.go` · `import_format_steps_test.go` · `import_staging_steps_test.go` | import picker, row table, info block, skipped list, Postbank bytes, staging |
| `categorise_steps_test.go` | categorise dialog, category picker, rule form and drafts |
| `import_write_steps_test.go` · `carry_steps_test.go` | writing the import, assertions, opening balance, new account; year carry |
| `scheduled_steps_test.go` · `scheduled_form_steps_test.go` | rule list and panels, delete; the new/edit form and its writes |
| `forecast_steps_test.go` | forecast charts, lines, milestone table |
| `reports_steps_test.go` | balance sheet, income statement, periods |
| `invariants_steps_test.go` | cross-cutting writes, the lock, visiting every view, signals |
| `leftover_steps_test.go` | phrasings added late across features (forecast legend, transfers, undeclared accounts); a home is still to be found |

## Step vocabulary (keep it small)

Given: `the journal "x"` · `… with appended text "…"` · `today is D` · `the terminal is W×H` ·
`a Postbank CSV …` · `EDITOR is a stub …` · `hledger is slowed by / starts failing / returns garbage`
When: `I start konto` · `I press K` · `I type T` · `I open "x.csv" in Import` ·
`I import "x.csv" categorising every row` · `I confirm the form`
Then: `I see T` · `I do not see T` · `the status bar shows T` · `the journal contains T` ·
`the journal is unchanged` · `every hledger check passes` · `"T" is coloured N` ·
`konto exits with code N` · `konto is still running`
