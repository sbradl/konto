# features — e2e acceptance tests (Gherkin)

User's-eye tests of the compiled `konto` binary. One `.feature` per spec area; each cites its
source in `@spec-N.N` / `@adr-NNNN` tags so every requirement is traceable.

| file | area |
|---|---|
| `startup` | preflight, journal path, first-run, parse error, too-small terminal, lock |
| `chrome` | tab/status bar, global keys, `?` popup, refresh, quit, responsive layout, ANSI-16 theme |
| `checks` | `● clean` indicator, `!` panel, every check + severity, unclassified WARN, `$EDITOR` jump |
| `dashboard` | stat cards, Accounts / Recent / Upcoming / spend chart, dropped panels |
| `accounts` | close (`c`) / reopen (`o`) |
| `import-flow` | picker, IBAN detection, row table, skipped list, watermark dedup, bad input |
| `import-categorise` | popup, once / new rule / add to rule, overlap refusal, category picker, transfers |
| `import-write` | `w` gating, written text, assertion block, opening balance, new account, after-write |
| `import-staging` | staged work survives view switches, `q` guard, discard |
| `import-format` | raw Postbank bytes: BOM, preamble, trailer, German numbers/dates |
| `scheduled` | list, panels, form (`n`), edit (`e`), delete (`d`), never-posts |
| `forecast` | chart per currency, milestones, watermark start, never-written |
| `reports` | period cycling, balance sheet Δ, income statement, boundaries, scrolling |
| `invariants` | cross-cutting product rules (checks after every write, manual xor import, …) |

## Tags

`@spec-N.N` `@adr-NNNN` traceability · `@proposed` spec is silent, scenario encodes the obvious
reading — confirm or change · `@needs-decision` two readings are plausible, blocks the test (none open) ·
`@wip` not implemented yet · `@integration` needs real hledger · `@slow`

## Harness (not built yet — proposal)

- Runner `godog`, `//go:build e2e`, `mise run test:e2e`. Drives the **compiled binary** in a PTY
  at a fixed size (default 120×40), renders bytes through `charmbracelet/x/vt`, asserts on
  screen text, cell colour (ANSI index), exit code, and the files on disk.
- Real pinned `hledger`; every scenario gets a temp dir with a copy of the named fixture.
- **Clock seam**: features assume `today is 2026-09-30` unless a step says otherwise → one
  test-only env (`KONTO_TODAY`); konto passes explicit dates to hledger everywhere.
- Table cells may contain `\n` (Gherkin escape) — used to append multi-line journal snippets.
- `the journal contains` compares whitespace-normalised lines; `the journal is unchanged` is a
  byte comparison against the state at `I start konto`.
- **CSV generator**: `a Postbank CSV "x" for IBAN … with closing balance … :` renders a full
  Postbank export (UTF-8 BOM, preamble, header, rows as given — newest first — `Kontostand`
  trailer) into the XDG download dir. No real bank data anywhere; `import-format` alone feeds
  raw bytes.
- `EDITOR`/`VISUAL` stubs and a `hledger` shim (version faking, slowing, failing, garbage output,
  invocation recording) live in the harness.

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

## Step vocabulary (keep it small)

Given: `the journal "x"` · `… with appended text "…"` · `today is D` · `the terminal is W×H` ·
`a Postbank CSV …` · `EDITOR is a stub …` · `hledger is slowed by / starts failing / returns garbage`
When: `I start konto` · `I press K` · `I type T` · `I open "x.csv" in Import` ·
`I import "x.csv" categorising every row` · `I confirm the form`
Then: `I see T` · `I do not see T` · `the status bar shows T` · `the journal contains T` ·
`the journal is unchanged` · `every hledger check passes` · `"T" is coloured N` ·
`konto exits with code N` · `konto is still running`
