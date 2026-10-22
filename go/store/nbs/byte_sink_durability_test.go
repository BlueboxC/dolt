// Copyright 2019 Dolthub, Inc.
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
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordedSinkFile struct {
	file interface {
		io.WriteCloser
		Sync() error
	}
	calls                       []string
	writeErr, syncErr, closeErr error
}

func (f *recordedSinkFile) Write(p []byte) (int, error) {
	f.calls = append(f.calls, "write")
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.file.Write(p)
}
func (f *recordedSinkFile) Sync() error {
	f.calls = append(f.calls, "sync")
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.file.Sync()
}
func (f *recordedSinkFile) Close() error {
	f.calls = append(f.calls, "close")
	err := f.file.Close()
	if f.closeErr != nil {
		return f.closeErr
	}
	return err
}

func TestBufferedFileByteSinkDurabilityBeforePublication(t *testing.T) {
	for _, failure := range []string{"none", "write", "sync", "close"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			sink, err := NewBufferedFileByteSink(dir, 4096, 2)
			require.NoError(t, err)
			file := &recordedSinkFile{file: sink.wr}
			injected := errors.New("injected " + failure)
			switch failure {
			case "write":
				file.writeErr = injected
			case "sync":
				file.syncErr = injected
			case "close":
				file.closeErr = injected
			}
			sink.wr = file
			_, err = sink.Write([]byte("complete table bytes"))
			require.NoError(t, err)
			destination := filepath.Join(dir, "published-table")
			err = sink.FlushToFile(destination)
			if failure == "none" {
				require.NoError(t, err)
				require.Equal(t, []string{"write", "sync", "close"}, file.calls)
				contents, readErr := os.ReadFile(destination)
				require.NoError(t, readErr)
				require.Equal(t, "complete table bytes", string(contents))
			} else {
				require.ErrorIs(t, err, injected)
				_, statErr := os.Stat(destination)
				require.ErrorIs(t, statErr, os.ErrNotExist)
				if failure == "write" {
					require.Equal(t, []string{"write", "close"}, file.calls)
				} else {
					require.Equal(t, []string{"write", "sync", "close"}, file.calls)
				}
			}
		})
	}
}
