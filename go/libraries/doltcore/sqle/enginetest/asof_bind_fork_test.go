// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/types"
	"github.com/dolthub/vitess/go/vt/sqlparser"
)

func TestForkAsOfPreparedBinding(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, prepared := range []bool{false, true} {
			t.Run(fmt.Sprintf("wire=%v/prepared=%v", wire, prepared), func(t *testing.T) {
				ok := []sql.Row{{types.NewOkResult(0)}}
				script := queries.ScriptTest{Name: "AS OF binds at execution", SetUpScript: []string{
					"CREATE TABLE t(pk INT PRIMARY KEY)", "INSERT INTO t VALUES(1)", "CALL DOLT_COMMIT('-Am','one')",
					"INSERT INTO t VALUES(2)", "CALL DOLT_COMMIT('-am','two')", "SET @rev='HEAD~1'",
				}, Assertions: []queries.ScriptTestAssertion{
					{Query: "SELECT COUNT(*) FROM t AS OF 'HEAD~1'", Expected: []sql.Row{{1}}},
					{Query: "SELECT COUNT(*) FROM t AS OF @rev", Expected: []sql.Row{{1}}},
					{Query: "PREPARE history FROM 'SELECT COUNT(*) FROM t AS OF ?'", SkipResultsCheck: true},
					{Query: "EXECUTE history USING @rev", Expected: []sql.Row{{1}}},
					{Query: "SET @rev='HEAD'", Expected: ok},
					{Query: "EXECUTE history USING @rev", Expected: []sql.Row{{2}}},
					{Query: "SET @rev='missing-fork-revision'", Expected: ok},
					{Query: "EXECUTE history USING @rev", ExpectedErrStr: "branch not found: missing-fork-revision"},
					{Query: "SET @rev=NULL", Expected: ok},
					{Query: "EXECUTE history USING @rev", ExpectedErr: sql.ErrInvalidAsOfExpression},
					{Query: "SET @rev='HEAD~1'", Expected: ok},
					{Query: "EXECUTE history USING @rev", Expected: []sql.Row{{1}}},
					{Query: "SELECT COUNT(*) FROM t", Expected: []sql.Row{{2}}},
					{Query: "SELECT pk FROM t AS OF :v1 WHERE pk=:v2", Bindings: map[string]sqlparser.Expr{"v1": sqlparser.NewStrVal([]byte("HEAD~1")), "v2": sqlparser.NewIntVal([]byte("1"))}, Expected: []sql.Row{{1}}},
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
