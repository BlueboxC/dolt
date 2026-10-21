# DOX.md — Dolt engine corrections

## Purpose

Own the minimal Dolt dependency fork used by Beads Continuity.

## Ownership

Root owns upstream attribution, the pinned base and patch policy. Upstream remains dolthub/dolt; preserve its Apache-2.0 license and existing APIs.

## Local Contracts

- Base engine changes on the official v2.5.1 commit d1f8f8e763864d26e4b0753647c97f283f06e404. Keep main available for upstream synchronization and publish engine fixes on codex/beads-engine-fixes.
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
- Matching primary-key changes on both branches may project an unchanged-type ancestor key with unique non-NULL values. Preserve ancestor history, ordinary secondary indexes, constraints and conflict-base reachability; unsupported layouts retain the existing refusal.
- Recursive merge bases combine every best common ancestor deterministically. Conflicting bases refuse explicitly. Three-dot diff APIs share this resolver without changing branch, working-set or tag references; immutable virtual objects may be materialized.
- Artifact address traversal retains virtual conflict-base commits during garbage collection and native backup. Rollback after feature writes requires the matching retained data and engine, rather than a binary-only downgrade.

- DOLT_LOG --all traverses all local branch heads through the existing deduplicating history walker. Preserve explicit revision/range exclusions, table filters, prepared options and ordinary HEAD-only reads.

## Work Guidance

Keep the fork limited to the verified engine corrections described in FORK.md. Beads separately qualifies its complete dependency tuple and native backup compatibility before installation.

## Verification

From go/, run the affected DiffTableFunction, DiffStatTableFunction, DiffSummaryTableFunction and UnscopedDiffSystemTable engine tests, including prepared variants. Run view queries in normal and prepared mode and retain the current MIN/MAX and MOD regressions. Use disposable databases. Report test-only dependency differences and environment failures separately from engine failures.

Run ForkMergeCorrections in normal/prepared modes, merge and artifact traversal packages, and a persisted conflict through full garbage collection, native backup/restore and a cold reopen.

## Child DOX Index

