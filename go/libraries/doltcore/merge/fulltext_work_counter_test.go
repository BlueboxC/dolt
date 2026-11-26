// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.

// Proposed test: copy into merge/fulltext_work_counter_test.go only through
// Root's reviewed fixture admission. This artifact has not been compiled or run.
package merge

import (
	"context"
	"fmt"
	"math/bits"
	"testing"

	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/fulltext"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
	vitesssqltypes "github.com/dolthub/vitess/go/sqltypes"
	"github.com/stretchr/testify/require"

	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb/durable"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema/typeinfo"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/index"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/dolt/go/store/types"
)

// Embedding preserves the complete StringType interface and its collation.
// Compare delegates unchanged; its only extra effect is recording real calls.
type r8CountedStringType struct {
	sql.StringType
	calls *uint64
}

var _ sql.StringType = (*r8CountedStringType)(nil)

func (typ *r8CountedStringType) Compare(ctx context.Context, left, right any) (int, error) {
	*typ.calls = *typ.calls + 1
	return typ.StringType.Compare(ctx, left, right)
}

// This is a count-lookup workload, not an insert-only complexity test.
// Fulltext getRowCount/updateGlobalCount use this same full-PK lookup shape.
// The legacy editor flushes, sorts and copies accumulated rows on IndexedAccess.
// Its real sort invokes StringType.Compare. The mutable primary-map editor
// instead answers the point lookup without repeatedly sorting the full table.
//
// The counter does NOT observe raw equality inside memory.columnsMatch, or
// copied bytes. We therefore keep an actual growing legacy-editor workload as
// a sensitivity control. A manual Compare or two-row test would not establish
// that this counter sees the repeated accumulated-table work.
func TestFulltextCountLookupWork(t *testing.T) {
	for _, n := range []int{128, 256, 512} {
		t.Run(fmt.Sprintf("N=%d", n), func(t *testing.T) {
			ctx := sql.NewEmptyContext()
			product, legacy, counted, calls := r8NewCountLookupFixture(t, ctx)
			budget := uint64(4 * n * bits.Len(uint(n+1)))

			// Both tables receive the SAME instrumented PrimaryKeySchema, and
			// both use the same operation sequence and independent row oracle.
			// The legacy table is deliberately retained as a positive sensor
			// control after the product port; no runtime seam is required.
			require.Same(t, counted, product.Schema(ctx)[0].Type)
			require.Same(t, counted, product.SqlSch[0].Type)
			require.Same(t, counted, legacy.Schema(ctx)[0].Type)

			legacyWork := r8ExerciseCountLookups(t, ctx, legacy, counted, calls, n)
			// Sorting each growing set needs at least this many comparisons;
			// the real workload also performs the post-insert lookup each time.
			sensitivityFloor := uint64(n * (n - 1) / 2)
			require.GreaterOrEqual(t, legacyWork, sensitivityFloor,
				"counter is not sensitive to accumulated-table work on the actual legacy editor")
			require.Greater(t, legacyWork, budget,
				"legacy sensitivity workload must exceed the declared product budget")

			productWork := r8ExerciseCountLookups(t, ctx, product, counted, calls, n)
			// With the reviewed mutable-map port the full-PK bound comparison
			// still uses the supplied SQL type. Zero is not accepted as proof.
			require.Greater(t, productWork, uint64(0),
				"product count-lookup counter inactive; do not classify zero as an optimization")

			// Also verify the persisted primary map, not only the editor view.
			persisted, err := product.ApplyToTable(ctx)
			require.NoError(t, err)
			data, err := persisted.GetRowData(ctx)
			require.NoError(t, err)
			rows, err := durable.ProllyMapFromIndex(data)
			require.NoError(t, err)
			iter, err := rows.IterAll(ctx)
			require.NoError(t, err)
			result, err := sql.RowIterToRows(ctx,
				index.NewProllyRowIterForMap(product.Sch, rows, iter, nil, sql.ColSet{}))
			require.NoError(t, err)
			r8AssertCountContents(t, result, n)

			t.Logf("N=%d legacy_key_comparisons=%d product_key_comparisons=%d budget=%d",
				n, legacyWork, productWork, budget)
			if productWork > budget {
				t.Errorf("FULLTEXT repeated count lookups exceed work budget: N=%d comparisons=%d budget=%d",
					n, productWork, budget)
			}
		})
	}
}

func r8NewCountLookupFixture(t *testing.T, ctx *sql.Context) (
	*fulltextTable, *memory.Table, *r8CountedStringType, *uint64,
) {
	t.Helper()
	vrw := types.NewMemoryValueStore()
	t.Cleanup(func() { require.NoError(t, vrw.Close()) })
	ns := tree.NewNodeStore(vrw.ChunkStore())
	root, err := doltdb.EmptyRootValue(ctx, vrw, ns)
	require.NoError(t, err)

	wordType := gmstypes.MustCreateStringWithDefaults(vitesssqltypes.VarChar, 64)
	wordInfo, err := typeinfo.FromSqlType(wordType)
	require.NoError(t, err)
	word, err := schema.NewColumnWithTypeInfo("word", 10, wordInfo, true,
		"", false, "", schema.NotNullConstraint{})
	require.NoError(t, err)
	count := schema.NewColumn("n", 11, types.UintKind, false, schema.NotNullConstraint{})
	sch := schema.MustSchemaFromCols(schema.NewColCollection(word, count))
	base, err := doltdb.NewEmptyTable(ctx, vrw, ns, sch)
	require.NoError(t, err)
	root, err = root.PutTable(ctx, doltdb.TableName{Name: "r8_work_counts"}, base)
	require.NoError(t, err)
	product, err := createFulltextTable(ctx, "r8_work_counts", root)
	require.NoError(t, err)

	// Keep the actual production factory's persisted table/schema/map state.
	// Replace only this test instance's SQL schema and memory metadata table.
	// This compiles before and after the port: no reference to new fields.
	pkSchema := sql.NewPrimaryKeySchema(product.SqlSch.Copy(), 0)
	baseType, ok := pkSchema.Schema[0].Type.(sql.StringType)
	require.True(t, ok)
	calls := new(uint64)
	counted := &r8CountedStringType{StringType: baseType, calls: calls}
	pkSchema.Schema[0].Type = counted
	product.SqlSch = pkSchema.Schema
	db := memory.NewDatabase("r8_work")
	product.GMSTable = memory.NewLocalTable(ctx, db, product.Name(), pkSchema, nil)
	legacy := memory.NewLocalTable(ctx, db, product.Name(), pkSchema, nil)
	require.Equal(t, baseType.Collation(), counted.Collation())
	return product, legacy, counted, calls
}

func r8ExerciseCountLookups(t *testing.T, ctx *sql.Context, table fulltext.EditableTable,
	keyType sql.StringType, calls *uint64, n int,
) uint64 {
	t.Helper()
	indexes, err := table.GetIndexes(ctx)
	require.NoError(t, err)
	require.Len(t, indexes, 1)
	require.Equal(t, "PRIMARY", indexes[0].ID())
	editor, ok := table.Inserter(ctx).(sql.ForeignKeyEditor)
	require.True(t, ok, "FULLTEXT requires an editor with indexed access")
	closed := false
	defer func() {
		if !closed {
			require.NoError(t, editor.Close(ctx))
		}
	}()
	editor.StatementBegin(ctx)
	*calls = 0
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("word%06d", i)
		lookup := sql.IndexLookup{
			Index: indexes[0],
			Ranges: sql.MySQLRangeCollection{{
				sql.ClosedRangeColumnExpr(key, key, keyType),
			}},
		}
		// Check the unseen key, then its pending insert. This exercises
		// misses and read-your-writes through the actual count-access path.
		require.Empty(t, r8CountLookupRows(t, ctx, editor, lookup))
		want := sql.Row{key, uint64(i + 1)}
		require.NoError(t, editor.Insert(ctx, want))
		require.Equal(t, []sql.Row{want}, r8CountLookupRows(t, ctx, editor, lookup))
	}
	require.NoError(t, editor.StatementComplete(ctx))
	require.NoError(t, editor.Close(ctx))
	closed = true
	work := *calls

	parts, err := table.Partitions(ctx)
	require.NoError(t, err)
	result, err := sql.RowIterToRows(ctx, sql.NewTableRowIter(ctx, table, parts))
	require.NoError(t, err)
	r8AssertCountContents(t, result, n)
	return work
}

func r8CountLookupRows(t *testing.T, ctx *sql.Context, editor sql.ForeignKeyEditor,
	lookup sql.IndexLookup,
) []sql.Row {
	t.Helper()
	indexed := editor.IndexedAccess(ctx, lookup)
	require.NotNil(t, indexed)
	parts, err := indexed.LookupPartitions(ctx, lookup)
	require.NoError(t, err)
	rows, err := sql.RowIterToRows(ctx, sql.NewTableRowIter(ctx, indexed, parts))
	require.NoError(t, err)
	return rows
}

func r8AssertCountContents(t *testing.T, rows []sql.Row, n int) {
	t.Helper()
	require.Len(t, rows, n)
	expected := make(map[string]uint64, n)
	for i := 0; i < n; i++ {
		expected[fmt.Sprintf("word%06d", i)] = uint64(i + 1)
	}
	actual := make(map[string]uint64, n)
	for _, row := range rows {
		require.Len(t, row, 2)
		key, keyOK := row[0].(string)
		count, countOK := row[1].(uint64)
		require.True(t, keyOK)
		require.True(t, countOK)
		_, duplicate := actual[key]
		require.False(t, duplicate, "duplicate output key %q", key)
		actual[key] = count
	}
	require.Equal(t, expected, actual)
}
