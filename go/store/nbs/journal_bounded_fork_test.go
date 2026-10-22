// Copyright 2026 Dolthub, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0.

package nbs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dherrors "github.com/dolthub/dolt/go/libraries/utils/errors"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestForkJournalFileBatchBoundsLiveChunks(t *testing.T) {
	ctx := context.Background()
	j := newTestJournalWriter(t, newTestFilePath(t))
	hashes := hash.NewHashSet()
	for h, cc := range randomCompressedChunks(65) {
		require.NoError(t, j.writeCompressedChunk(ctx, dherrors.FatalBehaviorError, cc))
		hashes.Insert(h)
	}
	require.NoError(t, j.flush(ctx, dherrors.FatalBehaviorError))
	entered, release := make(chan struct{}, 65), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var calls atomic.Int32
	var eg errgroup.Group
	t.Cleanup(func() { unblock(); _ = eg.Wait(); _ = j.Close() })
	source := journalChunkSource{journal: j}
	_, _, err := source.getManyCompressed(ctx, &eg, toGetRecords(hashes), func(context.Context, ToChunker) {
		entered <- struct{}{}
		<-release
		calls.Add(1)
	}, nil, &Stats{})
	require.NoError(t, err)
	for range 16 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("file workers did not start")
		}
	}
	select {
	case <-entered:
		t.Fatal("more than 16 file-backed chunks retained by paused callbacks")
	case <-time.After(100 * time.Millisecond):
	}
	unblock()
	require.NoError(t, eg.Wait())
	require.Equal(t, int32(65), calls.Load())
}

func TestForkJournalCanceledFileBatchReleasesClose(t *testing.T) {
	j := newTestJournalWriter(t, newTestFilePath(t))
	hashes := hash.NewHashSet()
	for h, cc := range randomCompressedChunks(40) {
		require.NoError(t, j.writeCompressedChunk(context.Background(), dherrors.FatalBehaviorError, cc))
		hashes.Insert(h)
	}
	require.NoError(t, j.flush(context.Background(), dherrors.FatalBehaviorError))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var eg errgroup.Group
	_, _, err := (journalChunkSource{journal: j}).getManyCompressed(ctx, &eg, toGetRecords(hashes), func(context.Context, ToChunker) {
		t.Error("canceled file read delivered a callback")
	}, nil, &Stats{})
	require.NoError(t, err)
	require.True(t, errors.Is(eg.Wait(), context.Canceled))
	done := make(chan error, 1)
	go func() { done <- j.Close() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close waited for canceled file readers")
	}
}
