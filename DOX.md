# DOX.md — Dolt engine corrections

## Purpose

Own the minimal Dolt dependency fork used by Beads Continuity.

## Ownership

Root owns upstream attribution, the pinned base and patch policy. Upstream remains dolthub/dolt; preserve its Apache-2.0 license and existing APIs.

## Local Contracts

- Base engine changes on the official v2.5.0 commit ea6ad5e1b7731e0c0df5a3252fef8e32dde4d26e. Keep main available for upstream synchronization and publish engine fixes on codex/beads-engine-fixes.
- Keep storage corrections in Dolt. Do not introduce Beads-specific schema, tasks, orchestration, configuration or private project data.
- Preserve upstream contributor tests and attribution. Remove a carried patch only after the corresponding upstream behavior passes its regression.
- Reject incompatible primary-key diffs explicitly instead of returning mispaired rows. Preserve supported same-key and physically compatible type changes.
- Indexed and filter-based dolt_diff history reads must respect the selected head's ancestry; explicit cross-revision APIs retain their existing behavior. This is query correctness, not a new authorization boundary.

## Work Guidance

Keep the fork limited to the two carried engine corrections described in FORK.md. Beads separately qualifies its complete dependency tuple and native backup compatibility before installation.

## Verification

From go/, run the affected DiffTableFunction, DiffStatTableFunction, DiffSummaryTableFunction and UnscopedDiffSystemTable engine tests, including prepared variants. Use disposable databases. Report test-only dependency differences and environment failures separately from engine failures.

## Child DOX Index
