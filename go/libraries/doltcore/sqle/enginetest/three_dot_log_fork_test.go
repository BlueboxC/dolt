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

func TestForkThreeDotLogExcludesEveryMergeBase(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(fmt.Sprintf("prepared=%v", prepared), func(t *testing.T) {
			script := queries.ScriptTest{Name: "criss-cross range excludes all common history", SetUpScript: []string{
				"CREATE TABLE t(id INT PRIMARY KEY)", "CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('b')",
				"INSERT INTO t VALUES(10)", "CALL DOLT_COMMIT('-am','A0')", "CALL DOLT_BRANCH('a0')",
				"CALL DOLT_CHECKOUT('b')", "INSERT INTO t VALUES(20)", "CALL DOLT_COMMIT('-am','B0')", "CALL DOLT_BRANCH('b0')",
				"CALL DOLT_CHECKOUT('main')", "CALL DOLT_MERGE('b0')",
				"CALL DOLT_CHECKOUT('b')", "CALL DOLT_MERGE('a0')",
				"SET @main=HASHOF('main')", "SET @b=HASHOF('b')", "SET @work=DOLT_HASHOF_DB('WORKING')",
				"SET @all=(SELECT COUNT(*) FROM DOLT_LOG('--all'))",
			}, Assertions: []queries.ScriptTestAssertion{
				{Query: "SELECT COUNT(*) FROM DOLT_MERGE_BASES('main','b')", Expected: []sql.Row{{2}}},
				{Query: "SELECT COUNT(*) FROM DOLT_LOG('main...b')", Expected: []sql.Row{{2}}},
				{Query: "SELECT COUNT(*) FROM DOLT_LOG('b...main')", Expected: []sql.Row{{2}}},
				{Query: "SELECT commit_hash=@main OR commit_hash=@b FROM DOLT_LOG('main...b')", Expected: []sql.Row{{true}, {true}}},
				{Query: "SELECT COUNT(*) FROM DOLT_LOG('main...main')", Expected: []sql.Row{{0}}},
				{Query: "SELECT COUNT(*) FROM DOLT_DIFF('main...b','t')", Expected: []sql.Row{{0}}},
				{Query: "SELECT (SELECT COUNT(*) FROM DOLT_LOG('--all'))=@all", Expected: []sql.Row{{true}}},
				{Query: "SELECT HASHOF('main')=@main,HASHOF('b')=@b,DOLT_HASHOF_DB('WORKING')=@work", Expected: []sql.Row{{true, true, true}}},
			}}
			h := newDoltHarness(t)
			defer h.Close()
			if prepared {
				enginetest.TestScriptPrepared(t, h, script)
			} else {
				enginetest.TestScript(t, h, script)
			}
		})
	}
}
