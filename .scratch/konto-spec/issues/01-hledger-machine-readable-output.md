# 01 hledger machine-readable output: what each subcommand provides

Type: research
Status: claimed
Context: /research subagent running (worktree-isolated); findings -> .scratch/konto-spec/research/01-hledger-output.md

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
