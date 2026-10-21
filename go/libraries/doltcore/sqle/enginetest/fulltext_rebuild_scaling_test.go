// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"strings"
	"testing"
)

func TestFulltextRebuildPrimaryMap(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(fmt.Sprintf("prepared=%v", prepared), func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			values := make([]string, 2000)
			for i := range values {
				values[i] = fmt.Sprintf("(%d,'word%d common','seed')", i+1, i+1)
			}
			script := queries.ScriptTest{Name: "two fulltext indexes with distinct and repeated tokens", SetUpScript: []string{
				"CREATE TABLE t(id INT PRIMARY KEY,a VARCHAR(100),b VARCHAR(100),FULLTEXT INDEX fa(a),FULLTEXT INDEX fb(b))",
				"INSERT INTO t VALUES" + strings.Join(values, ","),
				"INSERT INTO t VALUES(2001,NULL,NULL),(2002,'café common','apple apple'),(2003,'','')",
				"CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('other')",
				"UPDATE t SET a='mainword' WHERE id=1", "CALL DOLT_COMMIT('-am','main')",
				"CALL DOLT_CHECKOUT('other')", "UPDATE t SET b='otherword' WHERE id=2", "CALL DOLT_COMMIT('-am','other')", "CALL DOLT_MERGE('main')",
			}, Assertions: []queries.ScriptTestAssertion{
				{Query: "SELECT COUNT(*) FROM t", Expected: []sql.Row{{2003}}},
				{Query: "SELECT id FROM t WHERE MATCH(a) AGAINST('mainword')", Expected: []sql.Row{{1}}},
				{Query: "SELECT COUNT(*) FROM t WHERE MATCH(a) AGAINST('common')", Expected: []sql.Row{{2000}}},
				{Query: "SELECT id FROM t WHERE MATCH(b) AGAINST('apple')", Expected: []sql.Row{{2002}}},
				{Query: "SELECT id FROM t WHERE MATCH(b) AGAINST('otherword')", Expected: []sql.Row{{2}}},
				{Query: "SELECT a,b FROM t WHERE id=2001", Expected: []sql.Row{{nil, nil}}},
				{Query: "SELECT COUNT(*) FROM dolt_conflicts", Expected: []sql.Row{{0}}},
			}}
			if prepared {
				enginetest.TestScriptPrepared(t, h, script)
			} else {
				enginetest.TestScript(t, h, script)
			}
		})
		t.Run(fmt.Sprintf("keyless/prepared=%v", prepared), func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			script := queries.ScriptTest{Name: "keyless duplicates retain token and document counts", SetUpScript: []string{
				"CREATE TABLE t(a VARCHAR(100),b VARCHAR(100),FULLTEXT INDEX fa(a),FULLTEXT INDEX fb(b))",
				"INSERT INTO t VALUES('shared common','apples'),('shared common','apples'),(NULL,NULL)",
				"CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('other')",
				"INSERT INTO t VALUES('mainword common','apples')", "CALL DOLT_COMMIT('-am','main')",
				"CALL DOLT_CHECKOUT('other')", "INSERT INTO t VALUES('otherword common','berries')", "CALL DOLT_COMMIT('-am','other')", "CALL DOLT_MERGE('main')",
			}, Assertions: []queries.ScriptTestAssertion{
				{Query: "SELECT COUNT(*) FROM t", Expected: []sql.Row{{5}}},
				{Query: "SELECT COUNT(*) FROM t WHERE MATCH(a) AGAINST('shared')", Expected: []sql.Row{{2}}},
				{Query: "SELECT COUNT(*) FROM t WHERE MATCH(a) AGAINST('common')", Expected: []sql.Row{{4}}},
				{Query: "SELECT COUNT(*) FROM t WHERE MATCH(b) AGAINST('apples')", Expected: []sql.Row{{3}}},
				{Query: "SELECT COUNT(*) FROM t WHERE MATCH(b) AGAINST('berries')", Expected: []sql.Row{{1}}},
				{Query: "SELECT COUNT(IF(a IS NULL AND b IS NULL,1,NULL)) FROM t", Expected: []sql.Row{{1}}},
			}}
			if prepared {
				enginetest.TestScriptPrepared(t, h, script)
			} else {
				enginetest.TestScript(t, h, script)
			}
		})
	}
}
