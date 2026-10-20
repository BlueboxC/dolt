# Dolt engine fixes for Beads Continuity

This dependency fork starts from official Dolt v2.5.0 at `ea6ad5e1b7731e0c0df5a3252fef8e32dde4d26e`. It keeps corrections inside the storage engine rather than adding SQL workarounds to Beads.

- [Dolt #12013](https://github.com/dolthub/dolt/issues/12013): adapt elianddb's [PR #12017](https://github.com/dolthub/dolt/pull/12017), retaining its regression cases. Incompatible primary-key changes return an explicit error instead of silently pairing unrelated rows. Adapt the key comparison to 2.5.0's `ComparePrimaryKeys`; retain compatible physical-key/type changes and the existing drop/recreate handling. Preserve 2.5.0's crossed-column-tag refusal in the physical-descriptor fallback, with forward/reverse regression cases. Historical system-table scans retain their warning-and-stop contract.
- [Dolt #12104](https://github.com/dolthub/dolt/issues/12104): filter explicit history commits with the engine's existing ancestry API before constructing their row iterator. Both indexed and filter shortcuts use that iterator; full scans, current-head rows, merges and explicit revision comparisons keep their own semantics.

- [Dolt #12139](https://github.com/dolthub/dolt/issues/12139), reported by Alik1604: preserve every child-parent diff in `to_commit`/`from_commit` multi-hash lookups and full scans, including commits with shared parents. Enumerate parent edges directly instead of overwriting one child per parent; parent lookups share one ancestry walk and only load matching child tables; keep ancestry scope and historical key-change warnings.
- [Dolt #12138](https://github.com/dolthub/dolt/issues/12138), reported by johannesdb: create savepoints from the session's retained branch states, avoiding unrelated database create/drop races. Keep dirty flags, later-used branch rollback and transaction lifecycle notifications.

- [Dolt #12141](https://github.com/dolthub/dolt/issues/12141): pin the [SQL analyzer correction](https://github.com/BlueboxC/go-mysql-server/commit/6a5992f57d07ea00f2f190be98f74257146abd67) adapted from angelamayxie’s [GMS PR #4044](https://github.com/dolthub/go-mysql-server/pull/4044). Projected aliases receive column IDs even when unavailable to sibling projections; normal/prepared view filters preserve their own column bindings. The analyzer base retains the later MIN/MAX and decimal MOD fixes. Consumer main modules, including Beads, must repeat this Go replacement explicitly because replacements do not propagate through dependencies.

The official base already contains backup source-root error propagation ([#12057](https://github.com/dolthub/dolt/pull/12057)) and engine resource shutdown ([#12075](https://github.com/dolthub/dolt/pull/12075)). Those fixes are not duplicated here.

The license remains Apache-2.0. This fork is not an upstream release and does not change Dolt's storage format. Beads pins the exact engine commit, keeps its driver separately versioned, and performs its own compatibility and recovery qualification.

## Catalog, schema and remote follow-ups

- [#12122](https://github.com/dolthub/dolt/issues/12122): typed database disappearance and failed-creation cleanup support the analyzer's narrow INFORMATION_SCHEMA skips. Preserve the session-local SAVEPOINT fix and propagate unrelated trigger/root failures. Concurrent create/drop qualification is bounded correctness evidence, not a throughput benchmark.
- [PR #12152](https://github.com/dolthub/dolt/pull/12152), head `89d77b8be4995935c8795cee490bfdf2c390d916`: match CHECK references case-insensitively during schema merges; retain the contributor's mixed-case merge regressions.
- [PR #12153](https://github.com/dolthub/dolt/pull/12153), head `96b5ffd6796834be9b8d6c720c426e43e17a33df`, for [#11529](https://github.com/dolthub/dolt/issues/11529): use one remote-upstream cleanup helper from repository and SQL session state. Remove only branch tracking entries for the removed remote; preserve unrelated remotes/branches. Retain the contributor's CLI regressions.
- [#12100](https://github.com/dolthub/dolt/issues/12100) case 03: KV lookup joins skip NULL equality keys while retaining NULL-safe equality and left-outer extension. The analyzer fork carries the corresponding query, FULLTEXT, decimal and MOD corrections, with its own attribution and qualification limits.

The standalone CLI, external server and upstream release number remain distinct from Beads' embedded dependency tuple. Installing a new embedded tuple requires a fresh matched backup/readback; source tests alone are not live-database qualification.

Both independent consumer replacements remain required. The exact analyzer revision is recorded in go.mod.

## Merge-base and ancestor-key corrections

- [#12012](https://github.com/dolthub/dolt/issues/12012): when both branches select the same new primary-key tags, order and types, project the common ancestor into that key layout. Require unchanged key-column types, existing columns and unique non-NULL ancestor keys; retain ordinary secondary indexes and foreign-key constraints. Materialize an immutable virtual ancestor for conflict evidence. One-sided or incompatible keys, virtual columns, specialized/partial ancestor indexes and pre-existing ancestor artifacts keep the existing refusal. This is not arbitrary primary-key reconciliation.
- [#12050](https://github.com/dolthub/dolt/issues/12050): recursively combine all best common ancestors in stable hash order for merges and the shared three-dot diff/stat/summary resolver. Preserve branch/working/tag references and scalar merge-base APIs. Conflicting virtual bases refuse explicitly; automatic conflict-marker reconciliation is not implemented. Queries may create deterministic unreferenced objects, so logical reference preservation is not a claim of zero physical writes.
- Conflict metadata already names its base commit. Address traversal now retains that reference for garbage collection and native backup, including virtual ancestors not reachable from branch history. Existing storage serialization is unchanged. Older binaries do not know this additional reachability rule: use matched data/engine preimages for rollback after feature writes.

Qualification covers normal/prepared merges in both directions and squash mode, string/composite keys, secondary indexes, foreign keys, collision/NULL/incompatible-key refusals, two and three best ancestors, conflicting bases, unchanged logical references, and persisted conflict evidence through full GC, backup/restore and cold reopening. Contributor-provided repros are retained as regression scripts; these additions are independently implemented in this fork.
