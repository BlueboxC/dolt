# Dolt engine fixes for Beads Continuity

This dependency fork starts from official Dolt v2.6.0 at `05e6bceb1a210a7a8d770b74b8ef4825c165c0a8`. It keeps corrections inside the storage engine rather than adding SQL workarounds to Beads. This release and its SQL analyzer require Go 1.27.3.

The 2.6.0 integration reuses upstream changes already corresponding to carried fixes instead of applying them twice. It retains the fork's execution-time log options, remote-save ordering, non-unique ancestor lookup and storage/SQL regressions. The ORT merge-base implementation is reused with an explicit refusal when its virtual base contains conflicting cells, including the three-dot diff entry point; automatic reconciliation of such bases remains outside this fork's admitted behavior.

- [#12172 / PR #12173](https://github.com/dolthub/dolt/pull/12173): adapt reltuk's synchronization fix for file-backed byte sinks before GC publishes table files. Preserve the existing sink APIs and first-error handling. A narrow private writer capability permits deterministic write/sync/close failure tests; these tests and disposable filesystem round trips do not simulate physical power loss or certify every filesystem's durability.

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
- [PR #12153](https://github.com/dolthub/dolt/pull/12153), elianddb, updated head `741f6396580113cbc008793a5e5129340e5df5fc`, for [#11529](https://github.com/dolthub/dolt/issues/11529): use one remote-upstream cleanup helper from repository and SQL session state. Publish session removal only after repository-state saving succeeds. Provider, state-read, missing-remote and injected pre-write failures retain the live remote and tracking entries; successful removal preserves unrelated and similarly named remotes/branches. Retain the contributor's CLI regressions and the session failure-boundary regression. This does not make concurrent or partial repository-file writes transactional.
- [#12100](https://github.com/dolthub/dolt/issues/12100) case 03: KV lookup joins skip NULL equality keys while retaining NULL-safe equality and left-outer extension. The analyzer fork carries the corresponding query, FULLTEXT, decimal and MOD corrections, with its own attribution and qualification limits.

The standalone CLI, external server and upstream release number remain distinct from Beads' embedded dependency tuple. Installing a new embedded tuple requires a fresh matched backup/readback; source tests alone are not live-database qualification.

Both independent consumer replacements remain required. The exact analyzer revision is recorded in go.mod.

## Merge-base and ancestor-key corrections

- [#12012](https://github.com/dolthub/dolt/issues/12012): when both branches select the same new primary-key tags, order and types, project the common ancestor into that key layout. Require unchanged key-column types, existing columns and unique non-NULL ancestor keys; retain ordinary secondary indexes and foreign-key constraints. Materialize an immutable virtual ancestor for conflict evidence. One-sided or incompatible keys, virtual columns, specialized/partial ancestor indexes and pre-existing ancestor artifacts keep the existing refusal. This is not arbitrary primary-key reconciliation.
- [#12050](https://github.com/dolthub/dolt/issues/12050): recursively combine all best common ancestors through upstream 2.6.0's ORT resolver for merges and the shared three-dot diff/stat/summary resolver. Preserve branch/working/tag references and scalar merge-base APIs. Conflicting virtual bases refuse explicitly; automatic conflict-marker reconciliation is not implemented. Queries may create deterministic unreferenced objects, so logical reference preservation is not a claim of zero physical writes.
- Conflict metadata already names its base commit. Address traversal now retains that reference for garbage collection and native backup, including virtual ancestors not reachable from branch history. Existing storage serialization is unchanged. Older binaries do not know this additional reachability rule: use matched data/engine preimages for rollback after feature writes.

Qualification covers normal/prepared merges in both directions and squash mode, string/composite keys, secondary indexes, foreign keys, collision/NULL/incompatible-key refusals, two and three best ancestors, conflicting bases, unchanged logical references, and persisted conflict evidence through full GC, backup/restore and cold reopening. Contributor-provided repros are retained as regression scripts; these additions are independently implemented in this fork.


## Branch-log and stable-base follow-up

- [#12157](https://github.com/dolthub/dolt/issues/12157), reported by macneale4: honor `DOLT_LOG('--all')` by selecting every local branch head and using the existing history walker. Keep shared ancestors deduplicated, explicit revisions, two/three-dot ranges, `--not`, table and merge filters, and execution-time prepared options. Ordinary `DOLT_LOG()` still follows the current session head.
- The v2.5.1 base adds native log-table projection and the official file-locks update; the already-carried mixed-case CHECK correction is retained once. The session settings hook leaves MySQL behavior unchanged and does not qualify a DoltgreSQL integration.
- Raise the standalone module's `klauspost/compress` floor to 1.18.7; Beads already selected that version independently.
- [#12160](https://github.com/dolthub/dolt/issues/12160) is covered by the existing KV lookup NULL correction from #12100 case 03. Its exact reproduction and normal/prepared controls pass on the previously installed tuple; no duplicate analyzer patch is added.

## Keyless conflicts, added columns and Full-Text rebuilds

- [#12165](https://github.com/dolthub/dolt/issues/12165): a keyless conflict can refer to a base row already deleted on both sides. Skip its absent row before secondary-index updates instead of trying to build an index key from a nil tuple. Keep the existing artifact cleanup. Normal/prepared tests cover both resolution strategies, merge directions, Full-Text search and an ordinary secondary index.
- [#12166](https://github.com/dolthub/dolt/issues/12166): for an existing ancestor row with a newly added column, retain the other branch's value when one side is implicitly NULL. Preserve genuine conflicting inserts, different non-NULL edits, one-sided additions and existing default-value conflict behavior. The contributor's reproduction and these controls run in both directions and normal/prepared modes.
- [#12169](https://github.com/dolthub/dolt/issues/12169): use Dolt's existing mutable Prolly primary map while rebuilding Full-Text pseudo-tables. This removes the in-memory editor's repeated row scans on inserts and count lookups without introducing a new database, index format or dependency. Rebuilding remains proportional to the complete table, rather than incremental in the changed rows. Tests retain duplicate/keyless rows, NULL/empty text, Unicode and two Full-Text indexes. A bounded 500/1,000/2,000-row comparison checks identical query results; an independent 500-row comparison retains the exact working-root hash. Timings are local measurements, not general throughput guarantees.

## Full-Text ordinary-filter correctness

The pinned analyzer must distinguish MATCH lookups from ordinary equality/NULL/range filters on a Full-Text column. Preserve residual AND/OR semantics and keyed/keyless duplicates. Native Dolt runs the shared four-fixture, twenty-predicate normal/prepared regression alongside existing Full-Text and merge controls. The official 2.5.1 dependency pair reproduces the ordinary-filter error; this is separate from the earlier duplicate-document and derived-table fixes.

### GC callback contention and remote publication integrity (2026-10-22)

Dolt #12170 / [PR #12176](https://github.com/dolthub/dolt/pull/12176), by reltuk, head `15ef4ca724a8b3b303211da749d59aed92fa18f2`: resolve ranges and copy unflushed chunks under the lock, then read immutable flushed chunks outside it. Retain the contributor's mixed buffered/flushed, writer-progress and Close-draining tests. Adapt scheduling to 16 file workers so a slow consumer cannot retain the complete file batch; Close waits for registered readers and callbacks, while a read after Close returns the contributor's explicit error. The old callback-contention regression remains. Additional controls cover canceled readers and 65 file chunks with paused callbacks. Full NBS and focused race tests pass on macOS. Request metadata still scales with the supplied batch; no fixed total-RAM or general GC-throughput guarantee is claimed.

Adapted Dolt PR #12174 by reltuk (062e1e0e9c8fb2654d2423913340119ec67d92dd): validate upload body length/MD5 before publication, never replace an existing table file, refresh its pruning grace timestamp and verify chunk hashes before traversing references. Preserve Apache-2.0 attribution and the contributor’s HTTP, pruning and reference tests. Full remotesrv, file and NBS packages pass on macOS; Windows/Linux implementations are source/build checked separately, not native runtime certified here. The embedded Beads engine does not expose a remote HTTP service.

## Keyless secondary reader NULL contract

Adapt [PR #12171](https://github.com/dolthub/dolt/pull/12171) by angelamayxie: carry the NULL-safe equality mask into the keyless secondary iterator and reject ordinary NULL lookup keys there. A missing mask is ordinary equality. The earlier #12160 SQL fix remains in the KV mappers; this is complementary coverage of the lower reader API, verified with real Prolly primary/secondary maps and duplicate cardinality.


## Archive dictionary retention (#12177)

Deduplicate archive dictionaries by a SHA-256 of their raw content, rather than
the address of a decoded bundle. Reader-cache eviction can create new bundle
objects for the same dictionary; those objects no longer multiply output
dictionaries or stay pinned by the stream writer. Build compression dictionaries
only for actual Snappy recompression, preserving concurrent initialization.
The archive format and compressed chunk bytes remain compatible. Regression
coverage copies 600 chunks using 300 dictionaries through three generations,
forcing the existing 256-entry reader cache to evict dictionaries. This corrects
the retained-dictionary mechanism; it is not a fixed total-memory guarantee for
all GC or backup workloads. Based on quad341's report in dolthub/dolt #12177.


## Keyless conflict version schemas (#12186)

Deleting a keyless conflict uses the value descriptor of the row that supplied
its original hash: base for a removed ancestor row, ours for an added row. This
preserves identity across column type changes instead of writing base values
into current-schema fields. Normal and prepared regressions cover INT/VARCHAR
changes in both directions and retain duplicate cardinality and table contents.
The existing absent-row and Full-Text conflict protections remain unchanged.
Based on macneale4's supplied SQL reproduction in dolthub/dolt #12186.


## Commit-ancestor lookup cardinality (#12178)

The commit_hash index of dolt_commit_ancestors is non-unique. A merge commit
has several parent rows, so advertising uniqueness incorrectly selected the
server's max-one-row iterator. Direct and wire-protocol normal/prepared
regressions retain two-parent merges, ordinary commits and absent hashes.
No schema or index data migration is required: the correction changes the
system table's declared index contract.


## Small-journal memory reservations (#12179)

Allocate journal write buffers on demand, growing geometrically up to the
existing ceiling rather than reserving 5 MiB per opened journal. Keep that
logical ceiling independent of physical capacity, including oversized-record
refusal and flush offsets. Range-index maps grow with actual entries; a tiny
persisted index no longer reserves another 16,384-entry growth allowance.
The reflog, server database lifetime and maximum active-buffer size are unchanged.
This targets retained memory for small journals, not a fixed total-RSS promise
or automatic idle-database eviction. Existing journal reader/Close protections,
sync, recovery and on-disk formats remain intact.


## Three-dot log common-history exclusion (#12182)

Adapt macneale4's merged [PR #12182](https://github.com/dolthub/dolt/pull/12182),
head `098811895aac26c34792c1da8433a9c908499a8f`: exclude every best common
ancestor from `DOLT_LOG('A...B')` instead of choosing one. Preserve the fork's
existing argument handling, `--all` behavior and recursive virtual-base diff
resolver. Normal/prepared criss-cross regressions retain both directions,
identical heads, the expected two exclusive commits and unchanged logical refs.
The proposal's separate newest-base diff warning policy is not substituted for
the fork's already qualified virtual-base diff contract.

## Historical prepared table reads

Regression for [#12191](https://github.com/dolthub/dolt/issues/12191): qualify SQL PREPARE/EXECUTE and driver binds across direct/wire modes, alternating HEAD and its parent, NULL and invalid revisions. The analyzer replacement owns metadata-only deferral; execution still resolves each supplied revision.

## Journal burst retention

Adapt the buffer release from elianddb’s [PR #12194](https://github.com/dolthub/dolt/pull/12194) to our existing lazy allocation and range-index patch. Successful flushes release dynamic buffers above 64 KiB; empty flushes do no write. Fixed-capacity internal writers, the 5 MiB record ceiling and reader/Close coordination remain intact. Large repeated flushes trade renewed allocation for lower idle heap.

Consumer qualification preserves decimal types explicitly for the two wire/prepared literal-scale controls and the retained COALESCE fixture: the test driver serializes arbitrary-precision decimals as VARCHAR. Use legal DECIMAL(31,30) casts for wire scale controls; direct literal controls still exercise scale 40. This fixture adaptation is not a production coercion change.
