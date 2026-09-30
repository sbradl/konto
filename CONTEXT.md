# konto

konto is a terminal UI for browsing and editing an [hledger](https://hledger.org) journal —
single-user personal finance. It shells out to the `hledger` binary for all accounting
computation and maps hledger's output into the domain model defined here, which the rest of
the app speaks in.

## Ledger

**Ledger**:
A point-in-time snapshot of everything konto has loaded — accounts, transactions, price
directives, periodic rules, the last check run — plus metadata (journal path, transaction
count, hledger version, load time, health). Assembled from several independent `hledger`
calls; parts refresh independently.
_Avoid_: Journal (the on-disk file), Book, Database

**Journal**:
The plain-text hledger file(s) on disk, including anything pulled in via `include`. The
authoritative store; konto reads it through `hledger` and writes to it directly for edits.

**Main journal**:
The file the journal path resolves to; the rest of the journal is pulled in by `include`.
New accounts and directives are written here; a new transaction goes to the file holding its
account's latest transaction, else here.

## Accounts

**Account**:
A postable hledger account, identified by its full colon-separated name. konto holds
accounts as a flat list and does not model parent or aggregation accounts as entities.
In UI copy, "account" means only `assets:*` / `liabilities:*` (real money places).
_Avoid_: Ledger account

**Category**:
UI word for an `expenses:*` / `income:*` account — what money was spent on or came from.
In code and the journal it is an `Account`. Every transaction has one, except a transfer
or an opening balance; there is no catch-all or "uncategorized" (ADR 0008).
_Avoid_: Catch-all, Uncategorized

**Transfer**:
A transaction whose counterpart is another account rather than a category — moving money
between the user's own accounts (to savings, paying off a card). Counts toward neither
income nor expenses. Between two import-fed accounts each side imports its own leg against
the transfer clearing account; the two legs net it back to zero.

**Transfer clearing account**:
The single account both legs of a transfer between import-fed accounts post against, so each
import-fed account only ever holds transactions from its own CSV. A non-zero balance means a
transfer whose other leg is not imported yet. `assets:transfers`, source `Clearing`, created
with the journal; counts in net worth, shown in the account list, never closed, never an
import target, not forecast.

**Account source**:
Whether an account's entries come from manual entry (`Manual`), CSV import (`Import`), or
are the transfer clearing account (`Clearing`, exactly one, `assets:transfers`).
Applies only to `assets:*` / `liabilities:*` — never to categories.
An account has exactly one source — manual and import never mix, a hard product rule. Stored as a
`konto:source:` tag on the account's `account` directive in the journal. v1 has no
manual-entry UI (ADR 0010): import creates `Import` accounts; `Manual` exists only in the model.
_Avoid_: Account type (hledger's Asset/Liability/… classification)

**Unclassified account**:
An asset or liability account with no account-source tag. Never an import target, and
surfaced as a warning. Only arises from hand edits — konto creates
every account tagged (ADR 0010).

**Closed account**:
An account the user has retired, marked by a `konto:closed:<date>` tag on its `account`
directive. Only closable at zero balance and when no periodic rule references it; can be
reopened. Shown only in the account list, dimmed; counted in no total; excluded from the
forecast and the periodic-rule form. Importing a CSV whose IBAN belongs to a closed account
is refused until it is reopened.

**Account list**:
The flat list of all accounts with their balances and source — the one place closed accounts
appear, and where accounts are closed and reopened.
_Avoid_: Account overview, Account tree

## Amounts

**Amount**:
An exact quantity of a single commodity — an integer mantissa and a decimal scale,
mirroring hledger's wire representation. Never binary floating point.

**Multi-amount**:
A bundle of amounts in commodities that cannot be added together (hledger never sums unlike
commodities). The user holds EUR, PLN and CZK cash, so a balance is often a multi-amount;
shown as one figure plus a "+ N cur" count.
_Avoid_: Balance (may be a single amount or a multi-amount)

**Commodity**:
A currency or unit symbol together with its declared display style — symbol side, spacing,
decimal mark, digit grouping, precision.
UI copy calls it **currency**; "commodity" stays in code and docs.

**Valuation mode**:
Whether amounts show in their original commodity (`Native`, "at cost") or all converted to
one commodity via the latest price directives (`Market`, hledger's `-V`). Not surfaced in
v1 (EUR-only UI, ADR 0011); when it returns, one toggle shared by Dashboard and Reports. konto never computes valuation — it re-asks hledger.
_Avoid_: Cost (hledger "cost", `-B`, is conversion at transaction price — a different thing)

**Price directive**:
A dated `P` record stating one commodity's price in another. The input to market valuation.

## Transactions

**Transaction**:
A dated journal entry with a description and two or more postings that balance to zero.
Carries its source position (file, line) so konto can open it in `$EDITOR`.

**Posting**:
One line of a transaction: an account and a multi-amount, optionally a balance assertion,
status, and tags.

**Balance assertion**:
An `= <amount>` claim that an account's balance equals a known figure at that point. Each
import writes one, on the account's posting of the newest imported transaction, stating the
bank's closing balance for the export — konto's reconciliation mechanism. How a bank states
that balance is the bank format's concern.


## Periodic transactions

**Periodic rule**:
A `~` rule in the journal describing a recurring transaction (rent, salary). hledger has no
command to list these; konto parses them from journal text. Forecast-only: it generates
projected occurrences (via hledger) and never posts into the journal.
_Avoid_: Scheduled transaction, Recurring transaction, Template

**Occurrence**:
One projected dated instance of a periodic rule, computed by hledger. Feeds the forecast and
upcoming lists; never written to the journal.
_Avoid_: Instance, Posting (a periodic rule posts nothing)

## Import

**CSV row**:
One line of a bank CSV, converted by hledger into a candidate transaction, plus konto's
assessment. A row is either **skipped** (already in the journal, per the watermark — shown as
one undifferentiated list) or **new**. A new row is **matched** (an import rule gives its
category), **needs a category** (blocks the write), or **categorised once** (category set
by hand for this row only, no rule — also how a matched row is overridden).
_Avoid_: Uncategorised (ADR 0008 — that state never reaches the journal), Duplicate

**Opening balance**:
The transaction written on an account's first import: the bank's closing balance minus the
sum of the CSV's rows, against `equity:opening-balances`, dated the day before the oldest
row, with a balance assertion. Written even when zero — every import-fed account starts with
one. The only transaction in an import-fed account that is not a CSV row.

**Import rule**:
An `if` block in `categories.rules` mapping one or more description patterns (ORed lines)
to a category (or a transfer counterpart), tagged `konto-rule:<id>`. Starts narrow (the
full description, optionally shortened) and is widened by adding lines. Rules never overlap.

**Rule match stats**:
The live counts konto shows while building or widening a rule: rows it matches in this
file, and rows it would take from another rule (which blocks saving).

**Staged import**:
An import in progress: the CSV's new rows with their categorisations, plus any new account,
new categories and new or widened import rules it needs. Held by konto until written as one
unit — nothing reaches disk before that. Discarding it (e.g. quitting) leaves the journal
and rules untouched.
_Avoid_: Draft, Pending import

**Watermark**:
Per import-fed account, the date of its latest transaction in the journal. CSV rows before
it are skipped, after it are new, on it are compared by date + amount + description.
Derived, never stored (ADR 0009). An account with no transactions yet has none: on its first
import every row is new.
_Avoid_: Dedup marker, Cursor, `.latest`

**Bank format**:
A konto-built-in hledger rules preamble describing one bank's CSV layout (e.g. Postbank).
Assigned to an import-fed account by a `konto:format:<bank>` tag; the account itself is
recognised by its `konto:iban:<IBAN>` tag. v1 ships Postbank only.

## Forecast

**Forecast**:
Each open asset/liability account's journal balance projected forward by its periodic
rules' occurrences — nothing averaged or guessed. Projection starts from the account's
watermark, not today, so occurrences since its last import count too and "today" is itself a
projected figure. Never written to the journal.
_Avoid_: Scenario, Forecast driver, Trailing average (dropped, ADR 0007)

## Figures

**Net worth**:
The sum of the balances of all open accounts (assets plus liabilities).

**Net income**:
Income minus expenses over a period — the Dashboard shows it for the calendar month to date.
_Avoid_: Cash flow (hledger's `cashflow` is the change in cash accounts — a different figure;
a transfer to savings or a card payment moves cash but is neither income nor expense)

**Report period**:
The span a report covers, always ending today: month to date, quarter to date, year to date,
or all time. Chosen in Reports only; the Dashboard is always month to date.
_Avoid_: Period (alone — clashes with a periodic rule's period expression)

## Checks

**Check run**:
The result of running hledger's journal checks — a list of check issues, or clean. Backs
the `!` panel and the status-bar check status (`● clean` / `● 2 err · 3 warn`).

**Check issue**:
One problem found by a check: the check name, konto-assigned severity (`ERR` / `WARN`), a
message, and a file:line reference. hledger reports every issue as an error and stops at the
first; konto runs checks individually and assigns severity by check name.

**Report table**:
konto's shape for a balance sheet or income statement — rows each tagged with a kind
(section header, leaf, negative leaf, subtotal, grand total) that drives styling. Mapped
from hledger's compound balance report, not passed through.
