package index

import (
	"io"
	"testing"

	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb/durable"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/store/prolly"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/dolt/go/store/types"
	"github.com/dolthub/dolt/go/store/val"
	"github.com/stretchr/testify/require"
)

func TestKeylessSecondaryLookupNullEquality(t *testing.T) {
	ctx := t.Context()
	ns := tree.NewTestNodeStore()
	sch := schema.MustSchemaFromCols(schema.NewColCollection(schema.NewColumn("v", 0, types.IntKind, false)))
	_, err := sch.Indexes().AddIndexByColNames("idx_v", []string{"v"}, nil, schema.IndexProperties{})
	require.NoError(t, err)
	kd := val.KeylessTupleDesc
	vd := val.NewTupleDescriptor(val.KeylessCardType, val.Type{Enc: val.Int64Enc, Nullable: true})
	skd := val.NewTupleDescriptor(val.Type{Enc: val.Int64Enc, Nullable: true}, val.Type{Enc: val.Hash128Enc})
	vb := val.NewTupleBuilder(vd, ns)
	vb.PutUint64(0, 2)
	value, err := vb.Build(ctx, sharePool)
	require.NoError(t, err)
	key := val.HashTupleFromValue(sharePool, value)
	sb := val.NewTupleBuilder(skd, ns)
	sb.PutRaw(1, key.GetField(0))
	secondaryKey, err := sb.Build(ctx, sharePool)
	require.NoError(t, err)
	pri, err := prolly.NewMapFromTuples(ctx, ns, kd, vd, key, value)
	require.NoError(t, err)
	sec, err := prolly.NewMapFromTuples(ctx, ns, skd, val.NewTupleDescriptor(), secondaryKey, val.EmptyTuple)
	require.NoError(t, err)
	builder := keylessIndexImplBuilder{
		baseIndexImplBuilder: &baseIndexImplBuilder{idx: &doltIndex{id: "idx_v", tableSch: sch}},
		s:                    &durableIndexState{Primary: durable.IndexFromProllyMap(pri), Secondary: durable.IndexFromProllyMap(sec)},
	}
	lookup := val.NewTupleBuilder(skd.PrefixDesc(1), ns)
	nullKey, err := lookup.Build(ctx, sharePool)
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		safe  []bool
		count int
	}{
		{"ordinary equality", []bool{false}, 0},
		{"unspecified equality", nil, 0},
		{"null safe equality preserves duplicates", []bool{true}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gen, err := builder.NewSecondaryIter(false, 1, tc.safe)
			require.NoError(t, err)
			iter, err := gen.New(ctx, nullKey)
			require.NoError(t, err)
			count := 0
			for {
				k, v, err := iter.Next(ctx)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				require.Equal(t, key, k)
				require.Equal(t, value, v)
				count++
			}
			require.Equal(t, tc.count, count)
		})
	}
}
