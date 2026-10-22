// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
)

func TestForkCommitAncestorsLookupReturnsEveryParent(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, prepared := range []bool{false, true} {
			t.Run(fmt.Sprintf("wire=%v/prepared=%v", wire, prepared), func(t *testing.T) {
				script := queries.ScriptTest{Name: "commit lookup is non-unique", SetUpScript: []string{
					"CREATE TABLE t(id INT PRIMARY KEY)", "INSERT INTO t VALUES(1)", "CALL DOLT_COMMIT('-Am','base')",
					"CALL DOLT_BRANCH('other')", "INSERT INTO t VALUES(2)", "CALL DOLT_COMMIT('-am','main')",
					"SET @single=HASHOF('HEAD')", "CALL DOLT_CHECKOUT('other')", "INSERT INTO t VALUES(3)", "CALL DOLT_COMMIT('-am','other')",
					"CALL DOLT_CHECKOUT('main')", "CALL DOLT_MERGE('other')", "SET @merged=HASHOF('HEAD')",
				}, Assertions: []queries.ScriptTestAssertion{
					{Query: "SELECT parent_index FROM dolt_commit_ancestors WHERE commit_hash=@merged ORDER BY parent_index", Expected: []sql.Row{{int32(0)}, {int32(1)}}},
					{Query: "SELECT parent_index FROM dolt_commit_ancestors WHERE commit_hash=@single", Expected: []sql.Row{{int32(0)}}},
					{Query: "SELECT parent_index FROM dolt_commit_ancestors WHERE commit_hash='00000000000000000000000000000000'", Expected: []sql.Row{}},
				}}
				var h *DoltHarness
				if wire {
					h = newDoltServerTestHarness(t)
				} else {
					h = newDoltHarness(t)
				}
				defer h.Close()
				if prepared {
					enginetest.TestScriptPrepared(t, h, script)
				} else {
					enginetest.TestScript(t, h, script)
				}
			})
		}
	}
}
