// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
	"testing"
)

func TestKeylessNullLookupSQL(t *testing.T) {
	script := queries.ScriptTest{
		Name: "ordinary and NULL-safe keyless joins retain equality and duplicates",
		SetUpScript: []string{
			"CREATE TABLE t0(c0 INT)", "CREATE TABLE t1(c0 INT)", "CREATE UNIQUE INDEX i1 ON t1(c0)",
			"INSERT INTO t0 VALUES(NULL),(1),(2)", "INSERT INTO t1 VALUES(NULL),(1),(2)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{Query: "SELECT SUM(t0.c0=t1.c0) FROM t0,t1", Expected: []sql.Row{{float64(2)}}},
			{Query: "SELECT * FROM t0 JOIN t1 ON t0.c0=t1.c0 ORDER BY t0.c0", Expected: []sql.Row{{1, 1}, {2, 2}}},
			{Query: "SELECT * FROM t0 JOIN t1 ON t0.c0<=>t1.c0 ORDER BY t0.c0", Expected: []sql.Row{{nil, nil}, {1, 1}, {2, 2}}},
			{Query: "SELECT * FROM t0 LEFT JOIN t1 ON t0.c0=t1.c0 ORDER BY t0.c0", Expected: []sql.Row{{nil, nil}, {1, 1}, {2, 2}}},
			{Query: "INSERT INTO t1 VALUES(NULL)", Expected: []sql.Row{{gmstypes.NewOkResult(1)}}},
			{Query: "SELECT COUNT(*) FROM t0 JOIN t1 ON t0.c0=t1.c0", Expected: []sql.Row{{2}}},
			{Query: "SELECT COUNT(*) FROM t0 JOIN t1 ON t0.c0<=>t1.c0", Expected: []sql.Row{{4}}},
		},
	}
	t.Run("normal", func(t *testing.T) { h := newDoltHarness(t); defer h.Close(); enginetest.TestScript(t, h, script) })
	t.Run("prepared", func(t *testing.T) {
		h := newDoltHarness(t)
		defer h.Close()
		enginetest.TestScriptPrepared(t, h, script)
	})
}
