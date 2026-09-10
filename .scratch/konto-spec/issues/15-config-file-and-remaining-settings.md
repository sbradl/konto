# 15 Config file format and remaining settings

Type: grilling
Status: open

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
