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

## Accounts

**Account**:
A postable hledger account, identified by its full colon-separated name. konto holds
accounts as a flat list and does not model parent or aggregation accounts as entities.
_Avoid_: Category, Ledger account

**Account source**:
Whether an account's entries come from manual entry (`Manual`) or CSV import (`Import`).
An account is one or the other, never both — a hard product rule. Stored as a
`konto:source:` tag on the account's `account` directive in the journal.
_Avoid_: Account type (hledger's Asset/Liability/… classification)

**Unclassified account**:
A postable account with no account-source tag yet. Excluded from manual entry and from
import targets, and surfaced as a warning; konto asks the user to classify it the first
time the account is opened.

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

**Valuation mode**:
Whether amounts show in their original commodity (`Native`, "at cost") or all converted to
one commodity via the latest price directives (`Market`, hledger's `-V`). Toggled with `v`,
shared by Dashboard and Reports. konto never computes valuation — it re-asks hledger.
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
An `= <amount>` claim that an account's balance equals a known figure at that point. konto
writes one on the last posting of each imported transaction, from the bank's running-balance
column.

**Uncategorized transaction**:
A transaction konto flags as needing attention — a posting still lands in a catch-all
account, or the importer matched no rule. Shown with a `?` marker.

## Periodic transactions

**Periodic rule**:
A `~` rule in the journal describing a recurring transaction (rent, salary). hledger has no
command to list these; konto parses them from journal text. Never posts on its own — it
generates projected occurrences until one is committed.
_Avoid_: Scheduled transaction, Recurring transaction, Template

## Import

**Import source**:
A bank CSV/TSV file konto imports from, with its hledger rules file and its watermark.

**CSV row**:
One line of an import source, parsed into date, description, amount and running balance,
plus konto's assessment of which rule (if any) matches it.

**Import rule**:
An `if <pattern> … account2 <target>` block konto writes to a rules file to categorize
matching rows. The exact text is shown before it is saved.

**Rule match stats**:
The counts konto shows for a candidate rule: matches in this file, matches across the whole
journal, existing rule count, and rows still unmatched after saving.

**Watermark**:
The latest imported record date for an import source (hledger's `.latest.<file>`
convention). Rows on or before it are skipped as already-imported; konto surfaces the
skipped set as a list, not a bare count.
_Avoid_: Dedup marker, Cursor

## Forecast

**Forecast driver**:
An input line to the forecast projection, tagged `sched` (an exact figure from a periodic
rule) or `avg 6m` (a six-month trailing average). Inspectable, and for averages editable
in place.

**Scenario**:
A named set of assumptions the forecast runs under — `base`, `lean` (−15% spend), or
`3-month income gap`. Never written to the journal.

## Checks

**Check run**:
The result of running hledger's journal checks — a list of check issues, or clean. Backs
the `!` panel and the `● clean` health indicator.

**Check issue**:
One problem found by a check: the check name, konto-assigned severity (`ERR` / `WARN`), a
message, and a file:line reference. hledger reports every issue as an error and stops at the
first; konto runs checks individually and assigns severity by check name.

**Report table**:
konto's shape for a balance sheet or income statement — rows each tagged with a kind
(section header, leaf, negative leaf, subtotal, grand total) that drives styling. Mapped
from hledger's compound balance report, not passed through.
