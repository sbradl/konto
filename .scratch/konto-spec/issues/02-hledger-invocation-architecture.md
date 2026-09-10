# 02 hledger invocation architecture

Type: grilling
Status: open
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

Feeds **Go package and module layout**, **CSV import subsystem design**,
**Forecast computation design**, **Test seams and fixture strategy**.
