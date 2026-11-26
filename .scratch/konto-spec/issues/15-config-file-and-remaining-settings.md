# 15 Config file format and remaining settings

Type: grilling
Status: resolved

<!-- Deferred from ticket 10 (2026-09-10). No blocker. Grill when the build first needs a
persisted setting — likely when the `!` checks panel or the Import view lands. -->

## Question

Ticket 10 resolved journal-path resolution and the palette; these settings were deferred:

- **Config file**: format (TOML? YAML? a `.env`-style?) and location (XDG
  `~/.config/konto/config.toml`?). Precedence vs env vars and flags. Whether it is created
  on first run or purely optional.
- **Journal-check selection** (ticket 02): the on/off set for strict checks (`balanced`,
  `commodities`, `accounts` — default on) and opt-in checks (`ordereddates`,
  `uniqueleafnames`, `tags`, `recentassertions` — default off). Config shape.
- **Catch-all account name(s)** for `Transaction.Uncategorized` detection (ticket 03) —
  a single name, a list, or a pattern.
- **CSV import source directories** and per-source `.rules` locations (overlaps ticket 08 —
  may be resolved there instead).
- Whether the fixed ANSI palette gains a per-role override here (ticket 10 / ADR 0005 left
  this as "a later config feature").

## Answer

Grilled 2026-09-30. Headline: **konto has no config file in v1** — every setting on the list
dissolved.

### Where settings live

- Journal facts → the journal, as `account`-directive tags (same mechanism as
  `konto:source:` / `konto:closed:`). Machine settings → would be `~/.config/konto/config.toml`,
  but none remain.
- **No config file, no state file.** `internal/config` does journal-path resolution only:
  `--file` → `$LEDGER_FILE` → `~/.hledger.journal` (the "future config `journal` key" is
  dropped).

### Catch-all account → gone (ADR 0008)

- The user wants to be **forced to categorise**. No catch-all account (`expenses:unknown` /
  `income:unknown`), no uncategorized transactions ever enter the journal.
- **Import** writes only once **every** row has a category (pick existing or create new +
  rule). **No skip.** (Input to ticket 08.)
- **Entry**: category field required.
- Dropped: `Transaction.Uncategorized`, the Dashboard `?` marker and `c` categorize key, the
  "Uncategorized transaction" glossary term.

### UI terms

- **"category"** = `expenses:*` / `income:*`; **"account"** = `assets:*` / `liabilities:*`.
  Code and journal keep hledger's "account" for both.

### Journal checks → all, fixed

- The user wants data always correct: konto runs **every** hledger check, not
  configurable: basic (`parseable`, `autobalanced`, `assertions`), strict (`balanced`,
  `commodities`, `accounts`), plus `ordereddates`, `tags`, `payees`, `recentassertions`,
  `uniqueleafnames`.
- konto guarantees its own writes pass them:
  - `ordereddates` — insert transactions at their date position within the file, not append.
  - `tags` — auto-declare konto's tags (`tag konto:source`, `tag konto:closed`, …).
  - `payees` — auto-declare each new payee (`payee <description>`) on write.
  - `accounts` / `commodities` — declare new categories/accounts when created.
  - `uniqueleafnames` — refuse a new category/account whose last name part already exists.
  - `recentassertions` — satisfied by import's per-run `=` assertion.
- Severity: `payees` joins the WARN group.

### CSV file selection (input to ticket 08)

- Bubbles **`filepicker`**, filtered to `.csv`, opening in the **XDG download directory**
  (`~/Downloads` fallback). No "last directory" memory. Verify the v2 package path
  (`charm.land/bubbles/v2/filepicker`) at build time.

### Palette

- Fixed; no per-role override. The terminal theme is the only colour knob (ADR 0005 stands;
  the deferred override is not planned).
