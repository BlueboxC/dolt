# Dolt engine fixes for Beads Continuity

This dependency fork starts from official Dolt v2.5.0 at `ea6ad5e1b7731e0c0df5a3252fef8e32dde4d26e`. It keeps corrections inside the storage engine rather than adding SQL workarounds to Beads.

- [Dolt #12013](https://github.com/dolthub/dolt/issues/12013): adapt elianddb's [PR #12017](https://github.com/dolthub/dolt/pull/12017), retaining its regression cases. Incompatible primary-key changes return an explicit error instead of silently pairing unrelated rows. Adapt the key comparison to 2.5.0's `ComparePrimaryKeys`; retain compatible physical-key/type changes and the existing drop/recreate handling.
- [Dolt #12104](https://github.com/dolthub/dolt/issues/12104): filter explicit history commits with the engine's existing ancestry API before constructing their row iterator. Both indexed and filter shortcuts use that iterator; full scans, current-head rows, merges and explicit revision comparisons keep their own semantics.

The official base already contains backup source-root error propagation ([#12057](https://github.com/dolthub/dolt/pull/12057)) and engine resource shutdown ([#12075](https://github.com/dolthub/dolt/pull/12075)). Those fixes are not duplicated here.

The license remains Apache-2.0. This fork is not an upstream release and does not change Dolt's storage format. Beads pins the exact engine commit, keeps its driver separately versioned, and performs its own compatibility and recovery qualification.
