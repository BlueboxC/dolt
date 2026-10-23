package enginetest

import (
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	dtu "github.com/dolthub/dolt/go/libraries/doltcore/dtestutils"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/dsess"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/scriptgen/setup"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// This fork admits a clean virtual ancestor while retaining explicit refusal
// of conflicting virtual ancestors. Defaults must also survive a missing-cache rebuild in a different timezone.
func TestForkVirtualTimeDefaultRebuiltAfterGC(t *testing.T) {
	history := crissCross(
		[]string{"CREATE TABLE t (id INT PRIMARY KEY,a VARCHAR(100));", "INSERT INTO t VALUES(1,'seed');"},
		[]string{"ALTER TABLE t ADD COLUMN ts DATETIME(6) DEFAULT CURRENT_TIMESTAMP(6);"},
		[]string{"INSERT INTO t VALUES(2,'two');"},
		[]string{"UPDATE t SET a='y2';"},
		[]string{"UPDATE t SET a='x2';"},
	)
	checkForkTimeBaseAfterGC(t, history, "SELECT base_id,base_a,base_ts,our_a,their_a FROM dolt_conflicts_t ORDER BY base_id;")
}

func checkForkTimeBaseAfterGC(t *testing.T, history dtu.CrissCross, query string) {
	h := newDoltHarness(t)
	defer h.Close()
	h.Setup(setup.MydbData)
	e := mustNewEngine(t, h)
	defer e.Close()
	run := func(ctx *sql.Context, q string) []sql.Row {
		_, iter, _, err := e.Query(ctx, q)
		require.NoError(t, err, q)
		rows, err := sql.RowIterToRows(ctx, iter)
		require.NoError(t, err, q)
		return rows
	}
	ctx := enginetest.NewContext(h)
	for _, q := range history.Script() {
		run(ctx, q)
	}
	merging := enginetest.NewSession(h)
	for _, q := range []string{"SET @@dolt_allow_commit_conflicts = 1;", "CALL DOLT_CHECKOUT('x');", "CALL DOLT_MERGE('y', '-m', 'merge y into x');"} {
		run(merging, q)
	}
	if query == "" {
		query = "SELECT * FROM dolt_conflicts_t ORDER BY dolt_conflict_id;"
	}
	before := run(merging, query)
	require.NotEmpty(t, before)

	db, ok := h.provider.BaseDatabase(merging, "mydb")
	require.True(t, ok)
	ddb := db.(dsess.SqlDatabase).DbData().Ddb
	vrw, ns := ddb.ValueReadWriter(), ddb.NodeStore()
	meta := conflictMetadata(t, merging, ddb, "x")
	var mergeBases []hash.Hash
	for _, row := range run(merging, "SELECT * FROM dolt_merge_bases('x', 'y');") {
		mergeBases = append(mergeBases, hash.Parse(row[0].(string)))
	}
	assert.ElementsMatch(t, mergeBases, meta.MergeBases)
	cached, err := doltdb.LoadConflictBaseRoot(merging, vrw, ns, meta)
	require.NoError(t, err)
	cachedHash, err := cached.HashOf()
	require.NoError(t, err)

	run(merging, "CALL DOLT_GC();")
	stored, err := vrw.ReadValue(merging, meta.BaseRootIsh)
	require.NoError(t, err)
	require.NotNil(t, stored, "the fork retains referenced virtual merge bases")
	for _, mergeBase := range meta.MergeBases {
		source, err := vrw.ReadValue(merging, mergeBase)
		require.NoError(t, err)
		require.NotNil(t, source, "garbage collection keeps the merge bases")
	}

	after := enginetest.NewSession(h)
	run(after, "CALL DOLT_CHECKOUT('x');")
	run(after, "SET @@dolt_dont_merge_json = 1;")
	run(after, "SET time_zone='-07:00';")
	assert.Equal(t, before, run(after, query))
	// Exercise the missing-cache fallback without removing the retained object.
	missing := meta
	missing.BaseRootIsh = hash.Of([]byte("missing cached virtual base"))
	rebuilt, err := doltdb.LoadConflictBaseRoot(after, vrw, ns, missing)
	require.NoError(t, err)
	rebuiltHash, err := rebuilt.HashOf()
	require.NoError(t, err)
	assert.Equal(t, cachedHash, rebuiltHash)
	assert.Equal(t, []sql.Row{{"-07:00"}}, run(after, "SELECT @@time_zone;"))
	stale, err := dtu.StaleIndexes(after, rebuilt)
	require.NoError(t, err)
	assert.Empty(t, stale)

	run(after, "SET @@dolt_allow_commit_conflicts = 1;")
	run(after, "CALL DOLT_CONFLICTS_RESOLVE('--theirs', 't');")
	run(after, "CALL DOLT_COMMIT('-am', 'resolve merge of y');")
	assert.Equal(t, []sql.Row{{int64(0)}}, run(after, "SELECT count(*) FROM dolt_status;"))
}
