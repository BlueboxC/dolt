// Copyright 2022 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nbs

import (
	"context"
	dherrors "github.com/dolthub/dolt/go/libraries/utils/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestJournalCompressedCallbacksDoNotBlockWriter(t *testing.T) {
	ctx := context.Background()
	j := newTestJournalWriter(t, filepath.Join(t.TempDir(), "journal"))
	defer j.Close()
	data := randomCompressedChunks(40)
	var reqs []getRecord
	for h, cc := range data {
		require.NoError(t, j.writeCompressedChunk(ctx, dherrors.FatalBehaviorError, cc))
		reqs = append(reqs, getRecord{a: &h})
	}
	source := journalChunkSource{journal: j}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var eg errgroup.Group
	eg.SetLimit(2)
	done := make(chan error, 1)
	go func() {
		_, _, err := source.getManyCompressed(ctx, &eg, reqs, func(context.Context, ToChunker) { once.Do(func() { close(entered) }); <-release }, nil, nil)
		if err == nil {
			err = eg.Wait()
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("no callback")
	}
	wrote := make(chan error, 1)
	for _, cc := range randomCompressedChunks(1) {
		go func() { wrote <- j.writeCompressedChunk(ctx, dherrors.FatalBehaviorError, cc) }()
	}
	var writeErr error
	blocked := false
	select {
	case writeErr = <-wrote:
	case <-time.After(time.Second):
		blocked = true
	}
	close(release)
	require.NoError(t, <-done)
	if blocked {
		require.NoError(t, <-wrote)
		t.Fatal("writer waited on slow found callback")
	}
	require.NoError(t, writeErr)
}
