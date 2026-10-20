// Copyright 2026 Dolthub, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package env

import (
	"github.com/dolthub/dolt/go/libraries/doltcore/ref"
	"github.com/dolthub/dolt/go/libraries/utils/concurrentmap"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRemoveRemoteRetainsOtherTrackingAndPersists(t *testing.T) {
	env, fs := createTestEnv(true, false)
	state := env.RepoState
	state.Remotes.Set("origin", Remote{Name: "origin", Url: "file:///origin"})
	state.Remotes.Set("other", Remote{Name: "other", Url: "file:///other"})
	for _, name := range []string{"main", "feature"} {
		state.Branches.Set(name, BranchConfig{Remote: "origin", Merge: ref.MarshalableRef{Ref: ref.NewBranchRef(name)}})
	}
	kept := BranchConfig{Remote: "other", Merge: ref.MarshalableRef{Ref: ref.NewBranchRef("kept")}}
	state.Branches.Set("kept", kept)
	state.RemoveRemote(Remote{Name: "origin"})
	require.NoError(t, state.Save(fs))
	loaded, err := LoadRepoState(fs)
	require.NoError(t, err)
	_, exists := loaded.Remotes.Get("origin")
	require.False(t, exists)
	_, exists = loaded.Remotes.Get("other")
	require.True(t, exists)
	for _, name := range []string{"main", "feature"} {
		_, exists = loaded.Branches.Get(name)
		require.False(t, exists)
	}
	got, exists := loaded.Branches.Get("kept")
	require.True(t, exists)
	require.Equal(t, kept, got)
	UnsetRemoteUpstream(nil, "origin")
	empty := concurrentmap.New[string, BranchConfig]()
	UnsetRemoteUpstream(empty, "origin")
	require.Empty(t, empty.Snapshot())
}
