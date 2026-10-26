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

func TestForkAllLogHeadDecorations(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(fmt.Sprintf("prepared=%v", prepared), func(t *testing.T) {
			script := queries.ScriptTest{Name: "all log retains HEAD with aliased branch heads", SetUpScript: []string{
				"CREATE TABLE t(id INT PRIMARY KEY)", "CALL DOLT_COMMIT('-Am','head')", "CALL DOLT_BRANCH('same')",
				"SET @head=HASHOF('HEAD')", "SET @work=DOLT_HASHOF_DB('WORKING')",
			}, Assertions: []queries.ScriptTestAssertion{
				{Query: "SELECT refs LIKE '%HEAD -> main%' FROM DOLT_LOG('--all') WHERE message='head'", Expected: []sql.Row{{true}}},
				{Query: "SELECT COUNT(*)=COUNT(DISTINCT commit_hash) FROM DOLT_LOG('--all')", Expected: []sql.Row{{true}}},
				{Query: "SELECT message FROM DOLT_LOG('--all','--tables','t')", Expected: []sql.Row{{"head"}}},
				{Query: "SELECT COUNT(*) FROM DOLT_LOG('--all','--not','main')", Expected: []sql.Row{{0}}},
				{Query: "SELECT * FROM DOLT_LOG('main..same','--all')", ExpectedErr: sql.ErrInvalidArgumentDetails},
				{Query: "SELECT * FROM DOLT_LOG('main...same','--all')", ExpectedErr: sql.ErrInvalidArgumentDetails},
				{Query: "SELECT HASHOF('HEAD')=@head,DOLT_HASHOF_DB('WORKING')=@work", Expected: []sql.Row{{true, true}}},
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
