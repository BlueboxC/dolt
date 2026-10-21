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

func TestKeylessFulltextResolve(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, side := range []string{"ours", "theirs"} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("prepared=%v/%s/reverse=%v", prepared, side, reverse), func(t *testing.T) {
					setup := []string{
						"CREATE TABLE t (a VARCHAR(100), b INT DEFAULT 1, FULLTEXT INDEX fa(a), KEY ib(b))",
						"INSERT INTO t(a) VALUES ('seed')",
						"CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('x')",
						"UPDATE t SET a='bravo'", "CALL DOLT_COMMIT('-am','main')",
						"CALL DOLT_CHECKOUT('x')", "UPDATE t SET a='alpha'", "CALL DOLT_COMMIT('-am','x')",
					}
					if reverse {
						setup = append(setup, "SET autocommit=0", "CALL DOLT_MERGE('main')")
					} else {
						setup = append(setup, "CALL DOLT_CHECKOUT('main')", "SET autocommit=0", "CALL DOLT_MERGE('x')")
					}
					script := queries.ScriptTest{Name: "keyless deleted conflict retains fulltext and ordinary indexes", SetUpScript: setup,
						Assertions: []queries.ScriptTestAssertion{
							{Query: "SELECT COUNT(*) FROM dolt_conflicts", Expected: []sql.Row{{1}}},
							{Query: fmt.Sprintf("CALL DOLT_CONFLICTS_RESOLVE('--%s','t')", side), Expected: []sql.Row{{0}}},
							{Query: "SELECT COUNT(*) FROM dolt_conflicts", Expected: []sql.Row{{0}}},
							{Query: "SELECT a FROM t ORDER BY a", Expected: []sql.Row{{"alpha"}, {"bravo"}}},
							{Query: "SELECT a FROM t WHERE MATCH(a) AGAINST('alpha')", Expected: []sql.Row{{"alpha"}}},
							{Query: "SELECT a FROM t WHERE MATCH(a) AGAINST('bravo')", Expected: []sql.Row{{"bravo"}}},
							{Query: "SELECT a FROM t WHERE MATCH(a) AGAINST('seed')", Expected: []sql.Row{}},
							{Query: "SELECT a FROM t FORCE INDEX(ib) WHERE b=1 ORDER BY a", Expected: []sql.Row{{"alpha"}, {"bravo"}}},
						},
					}
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
}
