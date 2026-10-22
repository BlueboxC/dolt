// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package nbs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	dherrors "github.com/dolthub/dolt/go/libraries/utils/errors"
	"github.com/dolthub/dolt/go/store/chunks"
	"github.com/stretchr/testify/require"
)

func TestForkSmallJournalReservations(t *testing.T) {
	const count = 16
	ctx := context.Background()
	dir := t.TempDir()
	for i := 0; i < count; i++ {
		require.NoError(t, os.Mkdir(filepath.Join(dir, fmt.Sprint(i)), 0o700))
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	writers := make([]*journalWriter, 0, count)
	for i := 0; i < count; i++ {
		p := filepath.Join(dir, fmt.Sprint(i), "journal")
		w := newTestJournalWriter(t, p)
		writers = append(writers, w)
		c := ChunkToCompressedChunk(chunks.NewChunk([]byte(fmt.Sprintf("small retained row %d", i))))
		require.NoError(t, w.writeCompressedChunk(ctx, dherrors.FatalBehaviorError, c))
		require.NoError(t, w.commitRootHash(ctx, dherrors.FatalBehaviorError, c.Hash()))
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(writers)
	perJournal := max(int64(0), int64(after.HeapAlloc)-int64(before.HeapAlloc)) / count
	t.Logf("retained Go heap bytes per small journal: %d; total allocated bytes: %d", perJournal, (after.TotalAlloc-before.TotalAlloc)/count)
	for _, w := range writers {
		require.NoError(t, w.Close())
	}
	require.Less(t, perJournal, int64(1024*1024), "small databases must not reserve megabyte-sized buffer/index capacity eagerly")
	for i := 0; i < count; i++ {
		w, exists, err := openJournalWriter(ctx, filepath.Join(dir, fmt.Sprint(i), "journal"))
		require.NoError(t, err)
		require.True(t, exists)
		root, err := w.bootstrapJournal(ctx, true, nil, nil)
		require.NoError(t, err)
		c := ChunkToCompressedChunk(chunks.NewChunk([]byte(fmt.Sprintf("small retained row %d", i))))
		require.Equal(t, c.Hash(), root)
		got, err := w.getCompressedChunk(c.Hash())
		require.NoError(t, err)
		require.Equal(t, c.FullCompressedChunk, got.FullCompressedChunk)
		require.LessOrEqual(t, cap(w.buf), 64*1024, "reopening a small journal must not allocate a maximum-sized buffer")
		require.NoError(t, w.Close())
	}
}

func TestForkJournalBufferRetainsRecordLimit(t *testing.T) {
	ctx := context.Background()
	w := newTestJournalWriter(t, filepath.Join(t.TempDir(), "journal"))
	defer w.Close()
	buf, err := w.getBytes(ctx, dherrors.FatalBehaviorError, 64)
	require.NoError(t, err)
	copy(buf, []byte("small buffered write"))
	before := append([]byte(nil), w.buf...)
	_, err = w.getBytes(ctx, dherrors.FatalBehaviorError, int(journalWriterBuffSize)+1)
	require.Error(t, err)
	require.Equal(t, before, w.buf)
	require.NoError(t, w.flush(ctx, dherrors.FatalBehaviorError))
	got := make([]byte, len(before))
	_, err = w.readAt(got, 0)
	require.NoError(t, err)
	require.Equal(t, before, got)
}
