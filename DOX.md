# DOX.md — Dolt engine corrections

## Purpose

Own the minimal Dolt dependency fork used by Beads Continuity.

## Ownership

Root owns upstream attribution, the pinned base and patch policy. Upstream remains dolthub/dolt; preserve its Apache-2.0 license and existing APIs.

## Local Contracts

- Base engine changes on the official v2.5.0 commit ea6ad5e1b7731e0c0df5a3252fef8e32dde4d26e. Keep main available for upstream synchronization and publish engine fixes on codex/beads-engine-fixes.
- Keep storage corrections in Dolt. Do not introduce Beads-specific schema, tasks, orchestration, configuration or private project data.
- Preserve upstream contributor tests and attribution. Remove a carried patch only after the corresponding upstream behavior passes its regression.
- Reject incompatible explicit-revision primary-key diffs instead of returning mispaired rows. Historical system-table scans retain warning-and-stop behavior. Preserve supported same-key and physically compatible type changes.
- Indexed and filter-based dolt_diff history reads must respect the selected head's ancestry; explicit cross-revision APIs retain their existing behavior. This is query correctness, not a new authorization boundary.

- Diff-history scans and multi-hash lookups retain each distinct child-parent edge, independently of traversal order; selected commits remain ancestry-scoped.
- Savepoints snapshot the session's branch states without enumerating unrelated provider databases. Preserve rollback for branches first edited after the savepoint and retained dirty flags.

- SQL view projections retain a distinct column identity even for aliases unavailable to sibling projections. Pin the reviewed analyzer in go.mod and require consumer main modules to repeat the replacement; dependency replacements are not transitive.

- Catalog enumeration skips only typed database disappearance; failed creation cleans up its partial provider registration. Preserve real root/trigger failures and the current session database.
- CHECK references are case-insensitive; remote removal clears only matching tracking entries in both persistent and SQL session state.
- KV lookup equality skips NULL keys without changing NULL-safe equality or outer NULL extension.

## Work Guidance

Keep the fork limited to the verified engine corrections described in FORK.md. Beads separately qualifies its complete dependency tuple and native backup compatibility before installation.

## Verification

From go/, run the affected DiffTableFunction, DiffStatTableFunction, DiffSummaryTableFunction and UnscopedDiffSystemTable engine tests, including prepared variants. Run view queries in normal and prepared mode and retain the current MIN/MAX and MOD regressions. Use disposable databases. Report test-only dependency differences and environment failures separately from engine failures.

## Child DOX Index
