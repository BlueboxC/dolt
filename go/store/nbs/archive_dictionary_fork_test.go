// Copyright 2025 Dolthub, Inc.
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
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dolthub/dolt/go/store/chunks"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/gozstd"
	"github.com/stretchr/testify/require"
)

func forkOpenWrittenArchive(t *testing.T, w *ArchiveStreamWriter) archiveReader {
	t.Helper()
	_, name, err := w.Finish()
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, w.FlushToFile(path))
	f, err := newFileReaderAt(path, false)
	require.NoError(t, err)
	h, err := w.writer.getName()
	require.NoError(t, err)
	ar, err := newArchiveReader(context.Background(), f, h, uint64(f.sz), NewUnlimitedMemQuotaProvider(), openOpts{deepValidate: true}, &Stats{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ar.close()) })
	return ar
}

func forkArchiveDictionaryCount(ar archiveReader) int {
	ids := make(map[uint32]struct{})
	for i := 0; i < int(ar.footer.chunkCount); i++ {
		d, _ := ar.getChunkRef(i)
		if d != 0 {
			ids[d] = struct{}{}
		}
	}
	return len(ids)
}

// Revisit every dictionary after the 256-entry reader cache has evicted it.
// Copying the same data through successive GC/pull writers must not multiply dictionaries.
func TestForkArchiveDictionaryIdentitySurvivesCacheEviction(t *testing.T) {
	const dictionaries = 300
	ctx := context.Background()
	dir := t.TempDir()
	writer, err := NewArchiveStreamWriter(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Cancel() })
	hashes := make([]hash.Hash, 2*dictionaries)
	values := make(map[hash.Hash][]byte)
	for i := 0; i < dictionaries; i++ {
		raw := bytes.Repeat([]byte(fmt.Sprintf("dictionary-%04d-", i)), 64)
		bundle, err := NewDecompBundle(gozstd.Compress(nil, raw))
		require.NoError(t, err)
		cd, err := gozstd.NewCDict(raw)
		require.NoError(t, err)
		for pass := 0; pass < 2; pass++ {
			data := []byte(fmt.Sprintf("dictionary-%04d-chunk-%d-data", i, pass))
			c := chunks.NewChunk(data)
			hashes[pass*dictionaries+i] = c.Hash()
			values[c.Hash()] = data
			_, err = writer.AddChunk(NewArchiveToChunker(c.Hash(), bundle, gozstd.CompressDict(nil, data, cd)))
			require.NoError(t, err)
		}
	}
	reader := forkOpenWrittenArchive(t, writer)
	require.Equal(t, dictionaries, forkArchiveDictionaryCount(reader))
	for generation := 0; generation < 3; generation++ {
		next, err := NewArchiveStreamWriter(dir)
		require.NoError(t, err)
		t.Cleanup(func() { _ = next.Cancel() })
		for _, h := range hashes {
			c, err := reader.getAsToChunker(ctx, h, &Stats{})
			require.NoError(t, err)
			_, err = next.AddChunk(c)
			require.NoError(t, err)
		}
		reader = forkOpenWrittenArchive(t, next)
		require.Equal(t, dictionaries, forkArchiveDictionaryCount(reader), "copy generation %d", generation)
		for _, h := range hashes {
			data, err := reader.get(ctx, h, &Stats{})
			require.NoError(t, err)
			require.Equal(t, values[h], data)
		}
	}
}

func TestForkArchiveReadBuildsCompressionDictionaryOnlyOnDemand(t *testing.T) {
	raw := bytes.Repeat([]byte("dictionary-content"), 64)
	bundle, err := NewDecompBundle(gozstd.Compress(nil, raw))
	require.NoError(t, err)
	require.Nil(t, bundle.cDict)
	cd, err := gozstd.NewCDict(raw)
	require.NoError(t, err)
	data := []byte("dictionary-content-chunk")
	c := chunks.NewChunk(data)
	got, err := NewArchiveToChunker(c.Hash(), bundle, gozstd.CompressDict(nil, data, cd)).ToChunk()
	require.NoError(t, err)
	require.Equal(t, data, got.Data())
	require.Nil(t, bundle.cDict, "read-only and copy paths must not build a compression dictionary")
	first, err := bundle.compressionDictionary()
	require.NoError(t, err)
	require.NotNil(t, first)
	t.Run("concurrent reuse", func(t *testing.T) {
		for i := 0; i < 16; i++ {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				next, err := bundle.compressionDictionary()
				require.NoError(t, err)
				require.Same(t, first, next)
			})
		}
	})
}

func TestForkArchiveDictionaryContentIndependentOfCompression(t *testing.T) {
	raw := bytes.Repeat([]byte("same-raw-dictionary"), 128)
	first, err := NewDecompBundle(gozstd.CompressLevel(nil, raw, 1))
	require.NoError(t, err)
	second, err := NewDecompBundle(gozstd.CompressLevel(nil, raw, 9))
	require.NoError(t, err)
	require.NotSame(t, first, second)
	w, err := NewArchiveStreamWriter(t.TempDir())
	require.NoError(t, err)
	defer w.Cancel()
	a, n, err := w.writeDictionary(first)
	require.NoError(t, err)
	require.Positive(t, n)
	b, n, err := w.writeDictionary(second)
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Zero(t, n)
	require.Len(t, w.dictMap, 1)
}
