// Copyright 2026 Dolthub, Inc.
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

package dsess

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/dolt/go/libraries/doltcore/ref"
	"github.com/dolthub/dolt/go/libraries/utils/concurrentmap"
	"github.com/dolthub/dolt/go/libraries/utils/filesys"
)

type remoteRemovalProvider struct {
	DoltDatabaseProvider
	fs  filesys.Filesys
	err error
}

func (p remoteRemovalProvider) FileSystemForDatabase(string) (filesys.Filesys, error) {
	return p.fs, p.err
}

type remoteRemovalWriteFailure struct {
	filesys.Filesys
	err error
}

func (f remoteRemovalWriteFailure) WriteFile(string, []byte, os.FileMode) error {
	return f.err
}

func TestRemoveRemotePublishesSessionOnlyAfterSave(t *testing.T) {
	injected := errors.New("injected repository-state failure")
	for _, phase := range []string{"provider", "read", "missing remote", "save", "success"} {
		t.Run(phase, func(t *testing.T) {
			fs := filesys.NewInMemFS([]string{"/.dolt"}, nil, "/")
			remotes := concurrentmap.New[string, env.Remote]()
			branches := concurrentmap.New[string, env.BranchConfig]()
			for _, name := range []string{"origin", "origin-extra", "other"} {
				remotes.Set(name, env.Remote{Name: name, Url: "file:///" + name})
				branches.Set(name, env.BranchConfig{Remote: name, Merge: ref.MarshalableRef{Ref: ref.NewBranchRef("main")}})
			}
			state := env.RepoState{
				Head:    ref.MarshalableRef{Ref: ref.NewBranchRef("main")},
				Remotes: remotes, Branches: branches,
				Backups: concurrentmap.New[string, env.Remote](),
			}
			require.NoError(t, state.Save(fs))
			// Loading makes the session and persisted maps independent, as in a real session.
			disk, err := env.LoadRepoState(fs)
			require.NoError(t, err)
			if phase == "missing remote" {
				disk.Remotes.Delete("origin")
				require.NoError(t, disk.Save(fs))
			}
			before, err := fs.ReadFile("/.dolt/repo_state.json")
			require.NoError(t, err)
			provider := remoteRemovalProvider{fs: fs}
			switch phase {
			case "provider":
				provider.err = injected
			case "read":
				require.NoError(t, fs.DeleteFile("/.dolt/repo_state.json"))
			case "save":
				provider.fs = remoteRemovalWriteFailure{Filesys: fs, err: injected}
			}
			s := NewSessionStateAdapter(&DoltSession{provider: provider}, "db", remotes, branches, nil)
			err = s.RemoveRemote(context.Background(), "origin")
			if phase != "success" {
				require.Error(t, err)
				_, present := remotes.Get("origin")
				require.True(t, present, "a refused removal must preserve the live remote")
				_, present = branches.Get("origin")
				require.True(t, present, "a refused removal must preserve live upstream tracking")
				if phase == "provider" || phase == "save" {
					require.ErrorIs(t, err, injected)
				}
				if phase != "read" {
					after, readErr := fs.ReadFile("/.dolt/repo_state.json")
					require.NoError(t, readErr)
					require.Equal(t, before, after, "failure must not change persisted state")
				}
			} else {
				require.NoError(t, err)
				_, present := remotes.Get("origin")
				require.False(t, present)
				_, present = branches.Get("origin")
				require.False(t, present)
				after, readErr := env.LoadRepoState(fs)
				require.NoError(t, readErr)
				_, present = after.Remotes.Get("origin")
				require.False(t, present)
				_, present = after.Branches.Get("origin")
				require.False(t, present)
			}
			for _, name := range []string{"origin-extra", "other"} {
				_, present := remotes.Get(name)
				require.True(t, present, "unrelated remote")
				_, present = branches.Get(name)
				require.True(t, present, "unrelated tracking")
			}
		})
	}
}
