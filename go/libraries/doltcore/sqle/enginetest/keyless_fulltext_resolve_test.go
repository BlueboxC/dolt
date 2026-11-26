// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb/durable"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/dsess"
	"github.com/dolthub/dolt/go/store/prolly"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/dolt/go/store/val"

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

// TestForkKeylessConflictResolveNoPanic keeps a direct same-goroutine assertion
// for the nil deleted-row tuple. The complete normal/prepared matrix above stays.
func TestForkKeylessConflictResolveNoPanic(t *testing.T) {
	h := newDoltHarness(t)
	defer h.Close()
	e := enginetest.MustNewEngine(t, h)
	defer e.Close()
	ctx := enginetest.NewContext(h)
	for _, query := range []string{
		"CREATE TABLE t (a VARCHAR(100), b INT DEFAULT 1, FULLTEXT INDEX fa(a), KEY ib(b))",
		"INSERT INTO t(a) VALUES ('seed')",
		"CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('x')",
		"UPDATE t SET a='bravo'", "CALL DOLT_COMMIT('-am','main')",
		"CALL DOLT_CHECKOUT('x')", "UPDATE t SET a='alpha'", "CALL DOLT_COMMIT('-am','x')",
		"CALL DOLT_CHECKOUT('main')", "SET autocommit=0", "CALL DOLT_MERGE('x')",
	} {
		ctx = ctx.WithQuery(query)
		enginetest.RunQueryWithContext(t, e, h, ctx, query)
	}
	enginetest.TestQueryWithContext(t, ctx, e, h, "SELECT COUNT(*) FROM dolt_conflicts", []sql.Row{{1}}, nil, nil, nil)
	logForkKeylessPhysicalRows(t, ctx, "before resolve")
	require.NotPanics(t, func() {
		query := "CALL DOLT_CONFLICTS_RESOLVE('--theirs','t')"
		queryCtx := ctx.WithQuery(query)
		_, iter, _, err := e.QueryWithBindings(queryCtx, query, nil, nil, nil)
		require.NoError(t, err)
		_, err = sql.RowIterToRows(queryCtx, iter)
		require.NoError(t, err)
	}, "a deleted keyless conflict must not insert a nil tuple into its secondary index")
	logForkKeylessPhysicalRows(t, ctx, "after resolve")
	for _, statement := range []string{
		"EXPLAIN SELECT a FROM t ORDER BY a",
		"SELECT COUNT(*) FROM t",
		"SELECT a FROM t",
		"SELECT a,b FROM t ORDER BY b,a",
		"SELECT a FROM t IGNORE INDEX(fa,ib)",
		"SELECT a FROM t IGNORE INDEX(fa,ib) ORDER BY a",
		"SELECT a FROM t FORCE INDEX(ib) WHERE b=1 ORDER BY a",
	} {
		qctx := ctx.WithQuery(statement)
		_, iter, _, err := e.QueryWithBindings(qctx, statement, nil, nil, nil)
		require.NoError(t, err)
		rows, err := sql.RowIterToRows(qctx, iter)
		require.NoError(t, err)
		t.Logf("keyless diagnostic %s: %v", statement, rows)
	}
	logForkKeylessPhysicalRows(t, ctx, "after query probes")
	enginetest.TestQueryWithContext(t, ctx, e, h, "SELECT a FROM t ORDER BY a", []sql.Row{{"alpha"}, {"bravo"}}, nil, nil, nil)
	enginetest.TestQueryWithContext(t, ctx, e, h, "SELECT a FROM t FORCE INDEX(ib) WHERE b=1 ORDER BY a", []sql.Row{{"alpha"}, {"bravo"}}, nil, nil, nil)
}

// logForkKeylessPhysicalRows observes the fixture's stored and pending maps
// without materializing edits. SQL probes and the existing assertions stay separate.
func logForkKeylessPhysicalRows(t *testing.T, ctx *sql.Context, phase string) {
	t.Helper()
	session, ok := ctx.Session.(*dsess.DoltSession)
	require.True(t, ok, "physical-row diagnostic requires the native Dolt session")
	state, found, err := session.LookupDbState(ctx, ctx.GetCurrentDatabase())
	require.NoError(t, err)
	require.True(t, found)
	root := state.UnmaterializedWorkingRoot()
	require.NotNil(t, root)
	table, found, err := root.GetTable(ctx, doltdb.TableName{Name: "t"})
	require.NoError(t, err)
	require.True(t, found)
	data, err := table.GetRowData(ctx)
	require.NoError(t, err)
	storedCount, err := data.Count()
	require.NoError(t, err)
	rows, err := durable.ProllyMapFromIndex(data)
	require.NoError(t, err)
	mapCount, err := rows.Count()
	require.NoError(t, err)
	t.Logf("keyless physical %s: map=%s index_count=%d map_count=%d",
		phase, rows.HashOf(), storedCount, mapCount)
	dump := func(label string, iter prolly.MapIter, desc *val.TupleDesc, ns tree.NodeStore) {
		unique := 0
		for {
			key, value, err := iter.Next(ctx)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			require.NotEmpty(t, value)
			// This exact fixture has cardinality,a,b at stored ordinals0,1,2.
			a, err := tree.GetField(ctx, desc, 1, value, ns)
			require.NoError(t, err)
			b, err := tree.GetField(ctx, desc, 2, value, ns)
			require.NoError(t, err)
			t.Logf("keyless physical %s/%s: key=%x card=%d a=%v b=%v",
				phase, label, key, val.ReadKeylessCardinality(value), a, b)
			unique++
		}
		t.Logf("keyless physical %s/%s: iterall_unique=%d", phase, label, unique)
	}
	iter, err := rows.IterAll(ctx)
	require.NoError(t, err)
	_, desc := rows.Descriptors()
	dump("stored", iter, desc, rows.NodeStore())
	if writeSession := state.WriteSession(); writeSession != nil {
		pending, hasPending, err := writeSession.PendingRows(ctx, doltdb.TableName{Name: "t"})
		require.NoError(t, err)
		t.Logf("keyless physical %s: has_pending=%v readable_view=%v", phase, hasPending, pending != nil)
		if pending != nil {
			iter, err := pending.Primary.IterAll(ctx)
			require.NoError(t, err)
			_, desc := pending.Primary.Descriptors()
			dump("pending", iter, desc, pending.Primary.NodeStore())
		}
	}
}
