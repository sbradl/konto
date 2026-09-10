# 01 hledger machine-readable output: what each subcommand provides

Type: research
Status: resolved
Findings: .scratch/konto-spec/research/01-hledger-output.md

## Question

konto shells out to the `hledger` binary (pre-decided). Establish, from primary sources
(hledger manual, `hledger` source, `--help`):

- Which subcommands does konto need to back the six views + global chrome? Candidates:
  `balancesheet`/`bal`, `incomestatement`/`is`, `print`, `register`/`reg`, `check`,
  `prices`, `balance --forecast`, `accounts`, `commodities`, `import`.
- For each, which machine-readable output format is available and best to parse
  (`--output-format json`, `csv`, `tsv`; `print` has no json — what does it emit?) and
  **exactly what fields that output contains vs omits**. Specifically:
  - Does `print --output-format json` (or `print`) round-trip periodic `~` transactions,
    or does it expand them? How does konto read the raw `~` rule text for the Scheduled view?
  - Does `check --strict` output carry a `file:line` reference and a severity, or just a
    message on stderr with a non-zero exit? What's the exact format?
  - Does `balance`/`bal` json preserve multi-commodity amounts as separate entries (the
    EUR/PLN/CZK "+ 2 cur" case), and does `-V`/`--value` collapse them?
  - What does `balance --forecast` emit, and can forecast rows be distinguished from real
    postings in the output?
  - What does `hledger import --dry-run` output, and does it report the `.latest.<file>`
    dedup decisions?
- Minimum `hledger` version that provides the above (json output format, `check` subcommand
  shape, `--forecast`). State the version floor konto should require.

Findings feed **hledger invocation architecture** and **konto domain model and core types**.

## Answer

Full detail + verified command output + sources: `.scratch/konto-spec/research/01-hledger-output.md`
(verified against local `hledger 1.52.1`).

**Output formats**
- `-O json` works for `print`, `register`/`reg`, `aregister`/`areg`, `balance`/`bal`,
  `balancesheet`/`bs`, `incomestatement`/`is`, `cashflow`/`cf`.
- **No machine format** for `check`, `accounts`, `commodities`, `prices` — text only
  (`-O` is rejected outright).
- JSON amount quantities: use `decimalMantissa` + `decimalPlaces` (exact); `floatingPoint`
  is lossy — do not use it.

**Periodic `~` transactions**
- `print` (text or json) **drops `~` rules entirely**. `print --forecast` *expands* them
  into generated transactions tagged `_generated-transaction` (value = interval text, e.g.
  `~ monthly`) with `tsourcepos` giving the rule's `file:line`.
- **No hledger command lists periodic rules or returns their verbatim text.** konto must
  parse the journal file(s) itself for the Scheduled view's raw `~` rule display.

**`check --strict`**
- No machine format, no severity levels. Reports the **first failure only** → stderr, prose
  with a `file:line(:col)` prefix (prefix format varies per check), exit code 1. `Error:` only.
- Basic checks: parseable, autobalanced, assertions. `-s`/`--strict` adds balanced,
  commodities, accounts.
- To enumerate all problems konto must run checks individually, regex the stderr, then
  fix-rerun-repeat. (Feeds ticket 02's journal-check-panel design.)

**Balances / valuation**
- `bal -O json`: positional arrays `[[rows],[totals]]`; multi-commodity amounts kept
  separate (one amount per commodity — this is the "+2 cur" case). `-V` / `--value=end`
  collapses to a single EUR amount via the latest `P` directive.

**Forecast**
- `bal --forecast` aggregates only — forecast rows **cannot** be separated from real ones.
- Use `print --forecast` or `aregister --forecast` in json and filter on the
  `_generated-transaction` tag. Plain `register` cannot distinguish forecast rows;
  `aregister -O json` can (row[0] is the full txn with `ttags`). (Feeds ticket 09.)

**Import / dedup**
- `import --dry-run` prints only the would-be-added transactions in journal format. It does
  **not** list skipped rows or explain `.latest` decisions.
- `.latest.<file>` stores the latest seen date (repeated N times for N same-date records);
  import skips rows with `date < latest` plus the first N on that date.
- konto must build its own skipped-row list. (Feeds ticket 08.)

**Version floor: require `hledger >= 1.32` (2023-12-01).** Technical minimum is 1.20; 1.32
is recommended for forecast-txn source positions, the `tags` check, and import/commodity-style
fixes. konto parses `hledger --version` at startup, gates below the floor, and decodes JSON
tolerantly (optional/unknown fields — the schema has shifted in minor ways). An OpenAPI spec
for the JSON is linked from the hledger manual.
