// Copyright 2026 Dolthub, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package merge

import (
	"fmt"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/table/editor"
	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/go-mysql-server/sql"
	"sort"
)

// MergeBaseCommit combines every best common ancestor. Conflicting virtual
// bases refuse explicitly instead of silently choosing one candidate's data.
func MergeBaseCommit(ctx *sql.Context, resolver doltdb.TableResolver, left, right *doltdb.Commit, opts editor.Options) (*doltdb.Commit, error) {
	memo := make(map[[2]hash.Hash]*doltdb.Commit)
	var resolve func(*doltdb.Commit, *doltdb.Commit) (*doltdb.Commit, error)
	resolve = func(a, b *doltdb.Commit) (*doltdb.Commit, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ah, err := a.HashOf()
		if err != nil {
			return nil, err
		}
		bh, err := b.HashOf()
		if err != nil {
			return nil, err
		}
		if ah.String() > bh.String() {
			ah, bh = bh, ah
			a, b = b, a
		}
		key := [2]hash.Hash{ah, bh}
		if found := memo[key]; found != nil {
			return found, nil
		}
		ancestors, err := doltdb.GetCommitAncestors(ctx, a, b)
		if err != nil {
			return nil, err
		}
		type candidate struct {
			hash   hash.Hash
			commit *doltdb.Commit
		}
		bases := make([]candidate, len(ancestors))
		for i, c := range ancestors {
			actual, ok := c.ToCommit()
			if !ok {
				return nil, doltdb.ErrGhostCommitEncountered
			}
			h, err := actual.HashOf()
			if err != nil {
				return nil, err
			}
			bases[i] = candidate{h, actual}
		}
		if len(bases) == 0 {
			return nil, fmt.Errorf("commits have no common ancestor")
		}
		sort.Slice(bases, func(i, j int) bool { return bases[i].hash.String() < bases[j].hash.String() })
		virtual := bases[0].commit
		for _, next := range bases[1:] {
			base, err := resolve(virtual, next.commit)
			if err != nil {
				return nil, err
			}
			merged, err := mergeCommitsAtBase(ctx, resolver, virtual, next.commit, base, opts)
			if err != nil {
				return nil, err
			}
			if merged.HasMergeArtifacts() {
				return nil, fmt.Errorf("best common ancestors conflict; virtual merge base requires explicit resolution")
			}
			virtual, err = virtual.NewVirtualAncestor(ctx, merged.Root, next.commit)
			if err != nil {
				return nil, err
			}
		}
		memo[key] = virtual
		return virtual, nil
	}
	return resolve(left, right)
}
