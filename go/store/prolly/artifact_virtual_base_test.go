// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package prolly

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/dolt/go/store/val"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestArtifactWalkRetainsVirtualBase(t *testing.T) {
	ctx := context.Background()
	ns := tree.NewTestNodeStore()
	kd := val.NewTupleDescriptor(val.Type{Enc: val.Int16Enc})
	kb := val.NewTupleBuilder(kd, ns)
	kb.PutInt16(0, 1)
	key, err := kb.Build(ctx, sharedPool)
	require.NoError(t, err)
	base, theirs := hash.Of([]byte("virtual ancestor")), hash.Of([]byte("other commit"))
	metadata, err := json.Marshal(ConflictMetadata{BaseRootIsh: base})
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		kind   ArtifactType
		body   []byte
		retain bool
	}{{"conflict", ArtifactTypeConflict, metadata, true}, {"old empty metadata", ArtifactTypeConflict, []byte("{}"), false}, {"opaque constraint metadata", ArtifactTypeForeignKeyViol, []byte("opaque metadata"), false}} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := NewArtifactMapFromTuples(ctx, ns, kd)
			require.NoError(t, err)
			ed := m.Editor()
			require.NoError(t, ed.Add(ctx, key, theirs, tc.kind, tc.body, nil))
			m, err = ed.Flush(ctx)
			require.NoError(t, err)
			seen := map[hash.Hash]bool{}
			require.NoError(t, m.WalkAddresses(ctx, func(_ context.Context, h hash.Hash) error { seen[h] = true; return nil }))
			require.True(t, seen[theirs])
			require.Equal(t, tc.retain, seen[base])
			stop := errors.New("stop walk")
			require.ErrorIs(t, m.WalkAddresses(ctx, func(_ context.Context, h hash.Hash) error {
				if h == base {
					return stop
				}
				return nil
			}), func() error {
				if tc.retain {
					return stop
				}
				return nil
			}())
		})
	}
}
