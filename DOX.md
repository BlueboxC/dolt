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
- SQL remote removal publishes session-map changes only after repository-state saving succeeds; provider, read and save failures retain the live remote and tracking entries.
- KV lookup equality and the keyless secondary reader skip NULL keys unless the corresponding equality is explicitly NULL-safe. Preserve keyless duplicate cardinality and outer NULL extension.
- Matching primary-key changes on both branches may project an unchanged-type ancestor key with unique non-NULL values. Preserve ancestor history, ordinary secondary indexes, constraints and conflict-base reachability; unsupported layouts retain the existing refusal.
- Recursive merge bases combine every best common ancestor deterministically. Conflicting bases refuse explicitly. Three-dot diff APIs share this resolver without changing branch, working-set or tag references; immutable virtual objects may be materialized.
- Artifact address traversal retains virtual conflict-base commits during garbage collection and native backup. Rollback after feature writes requires the matching retained data and engine, rather than a binary-only downgrade.

- DOLT_LOG --all traverses all local branch heads through the existing deduplicating history walker. Preserve explicit revision/range exclusions, table filters, prepared options and ordinary HEAD-only reads.

- Conflict resolution ignores rows already deleted by both branches before touching secondary indexes; keyless Full-Text conflicts still clear their artifacts and retain search results.
- A nullable column added to an existing ancestor row treats implicit NULL as unchanged. Preserve genuine conflicting inserts and two non-NULL edits.
- Full-Text rebuilds use collation-aware Prolly primary maps for pseudo-table edits and count lookups. Preserve duplicates, NULLs, multiple indexes and persisted table contents; this remains a full rebuild.

- Full-Text filter qualification covers equality, NULL, ordinary index coexistence, AND/OR residuals and multiple Full-Text indexes in normal/prepared keyed and keyless queries.

- Journal compressed batches resolve ranges and copy unflushed bytes under the journal lock. Read immutable flushed ranges and invoke callbacks outside it with at most 16 file workers; do not buffer the complete file batch. Close waits for registered workers and their callbacks before closing the file; reads after closure return an error. Preserve keeper refusal and cancellation. This does not eliminate every GC pause.

- Remote upload readers reject length/checksum mismatches before table publication. Existing table files use no-replace publication and retain contents; refresh their pruning grace timestamp. Chunk references validate the content hash before address traversal. Preserve error propagation and pending-file protection.

- Archive stream writers deduplicate dictionary content by SHA-256, independently of reader-cache object identity. Copying/decompressing archived chunks creates no compression dictionary; create it only when recompressing Snappy input. Preserve archive format, chunk hashes and cancellation semantics.

- Keyless conflict artifact deletion hashes base rows with the base value descriptor and added rows with ours. Type/layout differences must not coerce historical rows into the current schema; preserve row cardinality and unrelated artifacts.

- Commit-ancestor commit_hash lookups are non-unique because merge commits retain one row per parent. Server and direct/prepared queries must return every selected edge.

- Journal buffers allocate on first use and grow to the existing per-record/batch ceiling. Small journals and range indexes do not reserve maximum capacity eagerly. Preserve flush offsets, sync, bootstrap and reader/Close coordination. Successful flushes release grown buffers above 64 KiB for dynamically sized writers; fixed-capacity internal writers retain their limit. Bursts allocate again after flushing; reflog and server database lifetimes are unchanged.

- Three-dot log ranges exclude every best common ancestor and its reachable history. Preserve symmetric ranges, --all, explicit exclusions and branch/working references; the fork virtual-base diff resolver keeps its existing contract.

- Historical prepared table reads resolve the bound revision on each execution; NULL and invalid revisions fail. Preparation uses the current schema for metadata and does not promise historical-only tables or columns.

## Work Guidance

- Byte sinks synchronize completed file contents before close and publication. A write, sync or close failure prevents BufferedFileByteSink from renaming its temporary file; preserve the first I/O failure and drain the background writer.

Keep the fork limited to the verified engine corrections described in FORK.md. Beads separately qualifies its complete dependency tuple and native backup compatibility before installation.

## Verification

From go/, run the affected DiffTableFunction, DiffStatTableFunction, DiffSummaryTableFunction and UnscopedDiffSystemTable engine tests, including prepared variants. Run view queries in normal and prepared mode and retain the current MIN/MAX and MOD regressions. Use disposable databases. Report test-only dependency differences and environment failures separately from engine failures.

Run ForkMergeCorrections in normal/prepared modes, merge and artifact traversal packages, and a persisted conflict through full garbage collection, native backup/restore and a cold reopen.

## Child DOX Index

