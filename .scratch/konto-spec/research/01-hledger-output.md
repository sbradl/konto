# 01 — hledger machine-readable output: what each subcommand provides

Research for konto (shells out to the `hledger` binary). Feeds **hledger invocation
architecture** and **konto domain model and core types**.

## Method & primary sources

- **Local binary**: `hledger 1.52.1, linux-x86_64` (installed at `/usr/bin/hledger`).
  All command output below was produced by running this binary against purpose-built
  test journals. `hledger --help` / `hledger <cmd> --help` text is the binary's own
  embedded manual and is treated as primary.
- **Manual**: <https://hledger.org/hledger.html> (the 1.52 manual), sections "Output format",
  "Commands".
- **Changelogs** (authoritative version history):
  - CLI: `https://raw.githubusercontent.com/simonmichael/hledger/master/hledger/CHANGES.md`
  - lib: `https://raw.githubusercontent.com/simonmichael/hledger/master/hledger-lib/CHANGES.md`
- **Release notes**: <https://hledger.org/relnotes.html>
- Current stable at time of writing: **1.52.3**; **hledger 2.0** is in preview (1.99.x).
  (relnotes.html; <https://github.com/simonmichael/hledger/releases>)

Where a fact could not be verified from a primary source it is called out as
**UNVERIFIED**.

---

## 1. Output-format support matrix

From the manual "Output format" table and confirmed against `hledger 1.52.1`
(`-O <fmt>` = `--output-format`; `-o FILE.<ext>` = `--output-file`):

| Command | txt | csv / tsv | json | html | sql | fods | beancount |
|---|---|---|---|---|---|---|---|
| `print` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `register` / `reg` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `aregister` / `areg` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `balance` / `bal` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `balancesheet` / `bs` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `balancesheetequity` / `bse` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `incomestatement` / `is` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `cashflow` / `cf` | ✅ | ✅ | ✅ | ✅ | – | ✅ | ✅ |
| `check` | **none** — no `-O`, no output on success | | | | | | |
| `accounts` | **none** — `-O` rejected ("Unknown flag: -O"); newline-delimited text only | | | | | | |
| `commodities` | **none** — `-O` rejected; newline-delimited text only | | | | | | |
| `prices` | **none** — `-O` rejected; emits `P` directives as text | | | | | | |
| `import` | text status line; with `--dry-run` emits journal-format text | | | | | | |

Verified rejections (1.52.1):

```
$ hledger -f t.journal accounts -Ojson
hledger: Error: Unknown flag: -O
$ hledger -f t.journal commodities -Ojson
hledger: Error: Unknown flag: -O
$ hledger -f t.journal prices -Ojson
hledger: Error: Unknown flag: -O
```

The manual: *"Our JSON is rather large and verbose, since it is a faithful
representation of hledger's internal data types. To understand its structure, read
the Haskell type definitions…"* There is an OpenAPI spec linked from the manual
(CLI CHANGES: "JSON output: link to OpenAPI spec").

**JSON numbers**: every amount quantity is an object
`{"decimalMantissa": <int>, "decimalPlaces": <int>, "floatingPoint": <number>}`.
Use `decimalMantissa` / `decimalPlaces` for exact decimal reconstruction —
`floatingPoint` is lossy. (Verified; CLI CHANGES 1.15: *"Numbers in JSON output now
provide a floating point Number as well as decimal mantissa & exponent."*)

---

## 2. Per-command detail

### 2.1 `print` — full transaction list (konto: manual-entry list, journal browsing)

**Best format to parse: `-O json`.** It is the only format that carries tags,
per-posting balance assertions, and source positions.

**`print -O json`** — top level is a JSON array of transaction objects. Keys
(verified 1.52.1):

```
tindex          int    1-based position in the parsed journal
tdate           "YYYY-MM-DD"
tdate2          "YYYY-MM-DD" | null      (secondary date)
tstatus         "Unmarked" | "Pending" | "Cleared"
tcode           string                  ( (...) transaction code )
tdescription    string                  ("payee | note" combined)
tcomment        string                  (transaction comment, newline-joined)
tprecedingcomment string
ttags           [[name, value], ...]     (parsed from comments; see forecast below)
tsourcepos      [ {sourceName,sourceLine,sourceColumn}, {…} ]   start & end pos
tpostings       [ posting objects ]
```

Posting object keys:

```
paccount            string
pamount             [ amount objects ]        one entry per commodity
pbalanceassertion   null | { baamount, bainclusive:bool, batotal:bool,
                             baposition:{sourceName,sourceLine,sourceColumn} }
pstatus             "Unmarked" | "Pending" | "Cleared"
pcomment            string
ptags               [[name,value], ...]
pdate, pdate2       null | "YYYY-MM-DD"
ptype               "RegularPosting" | "VirtualPosting" | "BalancedVirtualPosting"
poriginal           null | <posting>         (set when the posting was transformed)
ptransaction_       string (back-ref index)
```

Amount object: `{acommodity, aquantity:{decimalMantissa,decimalPlaces,floatingPoint},
acost, acostbasis, astyle:{ascommodityside("L"|"R"), ascommodityspaced, asdecimalmark,
asdigitgroups, asprecision, asrounding}}`.

**`print -O csv` / `-O tsv`** — one row **per posting** (not per transaction).
Columns (verified 1.52.1):

```
txnidx,date,date2,status,code,description,comment,account,amount,commodity,
credit,debit,posting-status,posting-comment
```

Omits: transaction tags, per-posting tags, balance assertions, source positions,
`ptype` (virtual vs real), `tprecedingcomment`. `amount` and split `credit`/`debit`
columns are both present. **Not sufficient for konto** — use JSON.

**`print -O sql`** — `insert into postings(...)` statements, same column set as CSV.

**`print` and the `= <balance>` assertion konto writes on CSV import**: round-trips
faithfully. `pbalanceassertion` carries `baamount`, `bainclusive` (`=*`),
`batotal` (`==`), and `baposition` (file:line:col). So konto can read reconciliation
state back out of `print -O json`.

#### Periodic (`~`) transactions and `print`

**`print` (default and `-O json`) does NOT round-trip `~` periodic transaction
rules. They are silently dropped from the output entirely.** Verified: a journal
with two `~` rules produces zero `~`-related output from plain `print`.

**`print --forecast`** *expands* the rules into concrete dated transactions, from
**after the latest regular transaction to 6 months after today** (default horizon;
`--forecast=PERIOD` overrides — *the equals is required*). Verified: `~ monthly rent`
generated `2025-02-02 … 2027-03-02` monthly entries when run on 2026-09-10.
A `~` rule whose occurrences all fall before the last real transaction generates
nothing under the default horizon.

Generated transactions are tagged. In `print --forecast -O json`:

```json
"ttags": [ ["_generated-transaction", "~ monthly"] ]
```

- The tag **name** is `generated-transaction`; the leading `_` is hledger's
  "hidden tag" prefix (normally suppressed in text output).
- The tag **value is the period-expression text of the originating rule**
  (`"~ monthly"`, `"~ every 15th of month"`, …). This is the closest thing to the
  raw rule text that any hledger output exposes.
- `tsourcepos` on the generated transaction points at the **`~` rule's own
  file:line** in the journal (start line = the `~` line, end line = last posting).
- `print --forecast --verbose-tags` (text) makes it visible as a comment:
  `; generated-transaction: ~ monthly`.
- `print --forecast -O csv` does **not** expose the tag (comment column empty);
  forecast rows are only distinguishable by having `txnidx` numbers appended after
  the real ones and empty status.

**How konto reads the raw `~` rule text for the Scheduled view** — there is no
hledger command that lists periodic-transaction rules. Options, in order of
robustness:

1. **Parse the journal file(s) directly** for lines beginning with `~` plus their
   indented posting block. hledger will not help here; this is unavoidable if konto
   needs the verbatim rule (interval expression, rule-level comment, tags).
   `hledger print -O json` gives the list of journal files via `tsourcepos.sourceName`
   and (from `hledger files`, a real subcommand) the full include set.
2. **Derive** rules from `print --forecast -O json`: group generated transactions by
   `_generated-transaction` tag value (the interval) + `tsourcepos` line, and read
   the amounts/accounts off the first generated instance. Loses anything that never
   generates an instance in the horizon, and loses the rule's exact start/end
   bounds.

Recommendation: **konto parses `~` blocks out of the journal text itself** for the
Scheduled view, and uses `print --forecast` only for the Forecast projection.

---

### 2.2 `register` / `reg` — flat posting register (konto: account drill-down)

**`reg -O json`** — array of 5-tuples (verified 1.52.1):

```
[ "YYYY-MM-DD" | null,     # posting date
  "YYYY-MM-DD" | null,     # secondary date
  description | null,
  <posting object>,        # same shape as print's posting object
  [ <amount object>, ... ] # running total, one entry per commodity
]
```

**`reg -O csv`** columns: `txnidx,date,code,description,account,amount,total`
(`amount`/`total` are rendered strings like `"800.00 EUR"`; multi-commodity totals
comma-joined in one cell).

**Forecast rows in `register` are NOT distinguishable.** The JSON tuple exposes only
the *posting* (`ptags` — empty for forecast) and never the parent transaction's
`ttags`. `reg --forecast -O csv` likewise carries no marker. If konto needs "is this
row a forecast" in a register-style view, it must use **`aregister`** (below) or
`print --forecast` instead.

---

### 2.3 `aregister` / `areg` — single-account register with running balance

**`areg -O json`** — array of 6-element rows (verified 1.52.1):

```
[ <full transaction object>,   # includes tindex, tpostings, ttags, tsourcepos …
  <transaction object>,        # the transaction as seen from this account
  <bool>,                      # (numbered/other-postings flag)
  [ "other:account", ... ],
  [ <amount> ... ],            # amount posted to the queried account
  [ <amount> ... ]             # running balance
]
```

Because row[0] is a **full transaction object**, `aregister --forecast -O json`
**DOES** expose `ttags = [["_generated-transaction","~ monthly"]]` on forecast rows.
This is the format to use for konto's Dashboard "recent activity per account" if it
must flag upcoming forecast items.

**`areg -O csv`** columns: `txnidx,date,code,description,target,otheraccounts,change,balance`
(UNVERIFIED exact header — not captured in this pass; JSON is preferred anyway).

---

### 2.4 `balance` / `bal` — the reporting workhorse (konto: Dashboard totals, custom reports)

**`bal -O json`** — a **positional** structure, not keyed objects (verified 1.52.1):

```
[
  [                                   # rows
    [ "full:account:name",
      "display name",
      <indent int>,
      [ <amount object>, ... ]         # ONE ENTRY PER COMMODITY, kept separate
    ],
    ...
  ],
  [ <amount object>, ... ]             # grand totals, one entry per commodity
]
```

- **Multi-commodity is preserved**: an account holding EUR + PLN + CZK yields a
  3-element `pamount` array; the grand-total array likewise lists all three
  separately (`hledger` does not add unlike commodities). Verified with a single
  account holding `100.00 EUR` + `200.00 PLN` → `[("EUR",100),("PLN",200)]`.
  This is konto's "`85748.24 + 2 cur`" case — the "+ 2 cur" count is
  `len(amounts) - 1`.

- **`-V` / `--market` / `--value=end` collapses** all commodities to a single
  valuation commodity (EUR here) using the latest applicable `P` directive on/before
  period end. Verified: `bal assets -V` → every row and the total become a single
  `EUR` amount. `-X EUR` / `--value=end,EUR` force the target commodity;
  `--value=then` values each posting at its own date; `--value=now` values at today.
  `-B` / `--cost` converts at transaction cost (`@`/`@@`), not market price.

- **Multi-period** (`-M`, `-Q`, `-Y`, `-p`): JSON gains per-period columns; CSV gets
  one column per period (`"account","2025-01","2025-02",...`). Multi-commodity cells
  in CSV are comma-joined strings.

**`bal -O csv`** — `"account","balance"` for a single period; balance cells are
rendered strings (`"3458.00 EUR"`; total row `"300.00 CZK, 3458.00 EUR, 500.00 PLN"`).

#### `balance --forecast`

`bal --forecast` adds the expanded `~` transactions into the totals for any period
that overlaps the forecast horizon. Verified: `bal expenses --forecast -e 2026-01-01`
returned `expenses:rent 8800.00 EUR` where the plain run returned nothing.

**Forecast contributions are NOT separable in `balance` output.** `balance`
aggregates postings; there is no per-transaction data and no "forecast" flag or
column in txt / csv / json. To show forecast-vs-actual, konto must either:
- run `balance` twice (with and without `--forecast`) and diff, or
- build the forecast view from `print --forecast -O json` / `aregister --forecast -O json`
  and aggregate client-side (filtering on the `_generated-transaction` tag).

`bal --budget` is a separate feature (compares actuals to `~` rule amounts in a
budget report) — relevant if konto ever wants budget vs actual, out of scope here.

---

### 2.5 `balancesheet` / `bs`, `incomestatement` / `is` (konto: Reports view)

**`bs -O json` / `is -O json`** — a "compound balance report" object (verified 1.52.1):

```
{
  "cbrTitle":     "Balance Sheet 2025-...",
  "cbrDates":     [ [ {tag:"Exact"|"Flex", contents:"YYYY-MM-DD"}, {…} ] ],
  "cbrSubreports": [
    [ "Assets", { prDates, prRows:[ {prrName, prrAmounts:[[amt…]…],
                                    prrTotal:[amt…], prrAverage:[amt…]} ],
                  prTotals:{…} }, <bool: increases total?> ],
    [ "Liabilities", {…}, <bool> ]
  ],
  "cbrTotals":    { … net total row … }
}
```

- `bs` subreports: **`Assets`, `Liabilities`**. `is` subreports:
  **`Revenues`, `Expenses`**. `bse` adds `Equity`.
- Same amount-object shape and same multi-commodity / `-V` behaviour as `bal`.
- **`bs -O csv` / `is -O csv`** flattens to `account,balance` style rows with
  subreport section headers interleaved — parseable but positional; JSON is cleaner.

---

### 2.6 `check` and `check --strict` (konto: journal-check `!` popup)

`check` produces **no stdout and exit code 0 on success**. On the **first** failure
it prints a human message to **stderr** and exits **non-zero (1)**. Verified.

- **There is no machine-readable mode.** No `-O`, no JSON, no structured output.
- **Only the first failing check / first failing item is reported**, then it stops.
  (`hledger check --help`: *"They are generally checked in the order they are shown
  here, and only the first failure will be reported."*)
- The message **does** contain a `file:line:` prefix, sometimes `file:line:col:`,
  followed by a source-excerpt with a caret underline and a prose explanation.
  Format is **not stable across check types**:

  Strict undeclared-commodity failure:
  ```
  hledger: Error: /path/t.journal:4:
  4 | P 2025-01-01 CZK 0.040 EUR

  Strict commodity checking is enabled, and
  commodity "CZK" has not been declared.
  ```

  Balance-assertion failure (adds a column, and a "calculated vs asserted" block):
  ```
  hledger: Error: /path/main.journal:32:42:
     | 2025-03-01 Coffee shop
  32 |     assets:bank:checking       -4.50 EUR = 995.50 EUR
     |                                          ^^^^^^^^^^^^
  ...
  Balance assertion failed in assets:bank:checking
  ...
  ```

  `ordereddates` failure (shows previous + offending entry):
  ```
  hledger: Error: /path/bad.journal:4:
  ...
  4 | 2024-12-01 y
    | ^^^^^^^^^^
  ...
  ```

- **Severity**: the literal token is always `Error:`. hledger `check` has **no
  "warning" level** — everything is pass/fail. (The `setup` command, 1.43+, does use
  warnings, but that's a different command.)

**Checks (from `hledger check --help`, 1.52.1):**

| Group | Checks | When run |
|---|---|---|
| **Basic** | `parseable`, `autobalanced`, `assertions` | automatically, by nearly every hledger command |
| **Strict** (`-s`/`--strict`) | adds `balanced`, `commodities`, `accounts` (and re-enables `assertions` even under `-I`) | only with `-s`/`--strict` |
| **On-demand** (name as arg) | `ordereddates`, `payees`, `tags`, `recentassertions`, `uniqueleafnames`, plus (1.50+) `balancednoautoconversion`/`balancedwithautoconversion`, `lots`, `basis` | `hledger check <name> ...` |

Custom checks are add-on scripts (`hledger-check-tagfiles`, `hledger-check-fancyassertions`).

Usage: `hledger check` (basic), `hledger check -s` (basic + strict),
`hledger check -s ordereddates recentassertions` (strict + named).

**konto implication**: to populate a checks panel with a *list* of problems, konto
must run `check` (or targeted `check <name>`) **once per check it cares about**,
capture stderr, and regex the `file:line(:col)?` prefix out of the first line
(`^hledger: Error: (?P<file>[^:]+):(?P<line>\d+):(?:(?P<col>\d+):)?`). Each run
still only reports one failure, so "fix, re-run, repeat" is the only way to
enumerate all instances of a given check. This is a real limitation to flag in the
architecture doc.

---

### 2.7 `prices` (konto: price directives / valuation plumbing)

- Text only. Emits `P` directives, one per line:
  `P 2025-01-01 PLN 0.23 EUR`. Directly re-parseable as journal input.
- Sort order is by commodity then date (not strictly chronological — verified:
  `PLN 2025-01-01`, `CZK 2025-01-01`, `PLN 2025-06-01`).
- `--infer-market-prices` adds prices inferred from transaction costs;
  `--infer-reverse-prices` (older `--infer-...`) adds inverses. Without flags, only
  declared `P` directives are shown.
- No `-O`. konto parses the `P <date> <commodity> <amount> <commodity>` lines.

### 2.8 `accounts` (konto: account list / navigation tree)

- Text only, one account per line, full names, depth-sorted. No `-O`.
- Useful flags: `--declared`, `--used`, `--undeclared`, `--tree`, `--directives`
  (emits `account` directives), `--types`.
- For a machine-friendly tree with balances, use **`bal -O json`** instead (its rows
  carry the full name + indent depth).

### 2.9 `commodities` (konto: commodity list)

- Text only, one symbol per line. No `-O`. (Verified: `CZK`, `EUR`, `PLN`.)
- Does not report display style / precision. For that, parse `commodity` directives
  from the journal, or read `astyle` from any `print -O json` amount.

### 2.10 `import` (konto: CSV import + dedup)

- Reads CSV/TSV/SSV per a rules file (`FILE.csv.rules` or `--rules-file`),
  **appends** new transactions to the first `-f` journal (must be journal format).
- Real run prints one status line to stdout:
  `imported N new transactions from bank.csv to /path/main.journal`, exit 0.
- **`import --dry-run`** prints `would import N new transactions from bank.csv:`
  followed by the new transactions **in journal (print) format** — valid journal
  text, re-parseable (`hledger import --dry-run f.csv | hledger -f- print`).
  Verified. Exit 0.
- **`--dry-run` does NOT enumerate skipped/duplicate rows and does NOT explain the
  `.latest` decision.** It shows only what *would be added*. There is no
  "skipped row" list, no per-row diagnostic, no count of what was filtered.
- `--catchup` marks everything as already imported without writing transactions
  (updates `.latest.*` only).

**`.latest.<file>` overlap detection** (from `hledger import --help`, which mirrors
the manual — quoted verbatim):

> For each FILE being imported from:
> 1. hledger reads a file named `.latest.FILE` in the same directory, if any. This
>    file contains the latest record date previously imported from FILE, in
>    YYYY-MM-DD format. If multiple records with that date were imported, the date
>    is repeated on N lines.
> 2. hledger reads records from FILE. If a latest date was found in step 1, any
>    records before that date, and the first N records on that date, are skipped.
> - After a successful import from all FILEs, without error and without `--dry-run`,
>   hledger updates each FILE's `.latest.FILE` for next time.

Verified: after importing two 2025-03-01 / 2025-03-03 rows, `.latest.bank.csv`
contained `2025-03-03`; a re-run with the same two rows + one 2025-03-07 row
imported only the 2025-03-07 row.

hledger explicitly declines to call this "deduplication" — it is a date-watermark
filter that assumes stable filename, stable dates, stable same-date order, newest
items = newest dates.

**konto implication**: to surface "skipped rows as a legible list" (a hard product
rule), konto **cannot rely on `import`** — it must implement its own preview:
read `.latest.<file>` (or track its own watermark), replay the same
`date < latest` + `first N on latest date` rule against the parsed CSV, and render
the skipped set itself. `import --dry-run` is only good for previewing the
*converted* form of the rows that *would* be added.

---

## 3. Version history & recommended floor

Versions confirmed from `hledger/CHANGES.md` + `hledger-lib/CHANGES.md` (release
dates from the changelog headers):

| Capability | First version | Date | Changelog evidence |
|---|---|---|---|
| `--forecast` (expand `~` into future txns) | **1.5** | 2017-12-31 | lib CHANGES 1.5: *"--forecast generates Ledger-style periodic transactions in the future"* |
| `generated-transaction` / `generated-posting` / `modified` tags on forecast/modifier output | **1.15** | 2019-09-01 | CLI CHANGES 1.15: *"Transactions and postings generated/modified by periodic transaction rules … are now marked with tags (`generated-transaction`, `generated-posting`, `modified`)"* |
| JSON output for `bs, cf, is, bal, print, reg` | **1.17** | 2020-03-01 | CLI CHANGES 1.17: *"bs, cf, is, bal, print, reg: support json output"*; lib 1.17: *"Moved JSON instances from hledger-web to hledger-lib … added ToJSON instances for all remaining data types"* |
| JSON numbers expose decimal mantissa/exponent + float | **1.15** | 2019-09-01 | CLI CHANGES 1.15: *"Numbers in JSON output now provide a floating point Number as well as decimal mantissa & exponent"* |
| `--forecast[=PERIODEXPR]` optional argument | **1.18** | 2020-06-07 | CLI CHANGES 1.18: *"The --forecast flag now takes an optional argument (--forecast=PERIODICEXPR)"* |
| JSON actually emitted by all commands that advertise it (bug fix) | **1.18** | 2020-06-07 | CLI CHANGES 1.18: *"commands allowing JSON output now actually produce JSON (#689)"* |
| `check` command (consolidates `check-dates`/`check-dupes`) + `-s`/`--strict` mode | **1.20** | 2020-12-05 | CLI CHANGES 1.20: *"check: A new command which consolidating the various check-* commands … runs the default, strict, or specified checks"*; *"strict mode: with -s/--strict, hledger requires that…"* |
| `payees` check | 1.21 | 2021-03-10 | CLI CHANGES 1.21 |
| `recentassertions` check | 1.27 | 2022-09-01 | CLI CHANGES 1.27 |
| `tags` check | ~1.29 | 2023-03-11 | CLI CHANGES 1.29: *"the check command now has a `tags` check"* |
| Strict mode also validates `~` periodic transactions | ~1.25 | 2022-03-04 | CLI CHANGES: *"Strict mode (`-s/--strict`) now also checks periodic transactions (`--forecast`)"* |
| `print` JSON includes source positions for `--forecast` transactions | ~1.29 | 2023-03-11 | CLI CHANGES: *"print's JSON output now also includes source positions for `--forecast` transactions"* |
| `import` writes `.latest` per data file; `import --dry-run`; `.latest.FILE` semantics | pre-1.5 … refined through 1.32 | — | `import` help text; CLI CHANGES 1.32.1 note about empty `.latest` files |
| JSON object keys sorted alphabetically; 2-space indent | 1.21 | 2021-03-10 | CLI CHANGES 1.21 |

### Recommended version floor: **require `hledger >= 1.32` (released 2023-12-01)**

Rationale:

- **Technical minimum is 1.20** (Dec 2020) — the first release where the `check`
  subcommand exists in its current shape, `-s/--strict` works, `--forecast=PERIOD`
  is available, and JSON output is reliable for `print`/`bal`/`reg`/`bs`/`is`.
  Anything older is missing the `check` command entirely.

- **1.32 is the recommended floor** because:
  - It is old enough (Dec 2023) to be in Debian 13, recent Ubuntu, Homebrew,
    Nixpkgs, and `ghcup`-installed toolchains — not a burden to require.
  - By 1.29 the `print --forecast` JSON carries `tsourcepos` for generated
    transactions (konto uses this to locate `~` rules) and the `tags` check exists.
  - 1.32 batches several import / commodity-style / `.latest` fixes that make CSV
    import behave predictably (commodity styles applied to imported amounts;
    `.latest` handling for multi-file imports).
  - JSON shape has been stable in all the ways konto depends on since ≥1.21
    (sorted keys, `decimalMantissa`/`decimalPlaces`, positional `bal` arrays).

- **Detect and gate**: konto should run `hledger --version` at startup, parse the
  `hledger X.Y` token, and refuse / warn below the floor. The JSON schema *has*
  shifted in minor ways over time (e.g. CLI CHANGES 1.42-era: an amount field was
  renamed to avoid clashing with a new `ptype` tag; `acostbasis` is recent), so
  konto's JSON decoding should be tolerant of unknown fields and defensive about
  optional ones rather than assuming an exact struct.

### Caveats / unverified

- Exact `aregister -O csv` column header not captured (JSON is recommended anyway).
- The precise release for "strict mode also checks periodic transactions" and
  "print JSON source positions for forecast" is bracketed to a 1–2 release window
  from changelog line position, not pinned to an exact header — treat as
  "available by 1.30".
- `fods` and `beancount` output formats exist but were not exercised (not needed by
  konto).
- All behavioural claims verified against **1.52.1 only**. Behaviour on the stated
  1.32 floor is inferred from changelogs, not re-tested against an old binary.
