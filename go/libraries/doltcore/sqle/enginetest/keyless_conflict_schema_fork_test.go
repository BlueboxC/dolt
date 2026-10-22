// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
)

func TestForkKeylessConflictDeletePreservesVersionSchema(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, change := range []struct{ name, create, alter string }{
			{"int-to-string", "CREATE TABLE t(c INT,d VARCHAR(20))", "ALTER TABLE t MODIFY c VARCHAR(20)"},
			{"string-to-int", "CREATE TABLE t(c VARCHAR(20),d VARCHAR(20))", "ALTER TABLE t MODIFY c INT"},
		} {
			t.Run(fmt.Sprintf("%s/prepared=%v", change.name, prepared), func(t *testing.T) {
				script := queries.ScriptTest{Name: change.name, SetUpScript: []string{
					change.create, "INSERT INTO t VALUES(1,'base'),(1,'base'),(2,'other')",
					"CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('other')",
					change.alter, "CALL DOLT_COMMIT('-am','ours')",
					"CALL DOLT_CHECKOUT('other')", change.alter, "CALL DOLT_COMMIT('-am','theirs')",
					"CALL DOLT_CHECKOUT('main')", "SET autocommit=0", "CALL DOLT_MERGE('other')",
					"SET @before_count=(SELECT COUNT(*) FROM t)",
				}, Assertions: []queries.ScriptTestAssertion{
					{Query: "SELECT COUNT(*)>0 FROM dolt_conflicts_t", Expected: []sql.Row{{true}}},
					{Query: "DELETE FROM dolt_conflicts_t", Expected: []sql.Row{{gmstypes.NewOkResult(4)}}},
					{Query: "SELECT COUNT(*) FROM dolt_conflicts_t", Expected: []sql.Row{{0}}},
					{Query: "SELECT COUNT(*)=@before_count FROM t", Expected: []sql.Row{{true}}},
					{Query: "SELECT DISTINCT CAST(c AS CHAR),d FROM t ORDER BY c", Expected: []sql.Row{{"1", "base"}, {"2", "other"}}},
					{Query: "SELECT COUNT(*) FROM dolt_conflicts", Expected: []sql.Row{{0}}},
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
}
