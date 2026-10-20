// Copyright 2026 Dolthub, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package doltdb

import (
	"context"
	"fmt"
	"github.com/dolthub/dolt/go/store/chunks"
	"github.com/dolthub/dolt/go/store/datas"
	"github.com/dolthub/dolt/go/store/hash"
)

// NewVirtualAncestor stores an unreferenced commit with one or two parents. It never
// updates a branch, working set or tag. Stable metadata makes repeated queries
// reuse the same object; conflict artifacts can retain its root by hash.
func (c *Commit) NewVirtualAncestor(ctx context.Context, root RootValue, other *Commit) (*Commit, error) {
	store, ok := c.vrw.(interface{ ChunkStore() chunks.ChunkStore })
	if !ok {
		return nil, fmt.Errorf("virtual merge bases require a chunk-backed value store")
	}
	left, err := c.HashOf()
	if err != nil {
		return nil, err
	}
	parents := []hash.Hash{left}
	lm, err := c.GetCommitMeta(ctx)
	if err != nil {
		return nil, err
	}
	date := lm.Committer.Date.Time()
	if other != nil {
		right, err := other.HashOf()
		if err != nil {
			return nil, err
		}
		parents = append(parents, right)
		rm, err := other.GetCommitMeta(ctx)
		if err != nil {
			return nil, err
		}
		if rm.Committer.Date.Time().After(date) {
			date = rm.Committer.Date.Time()
		}
	}
	identity := datas.CommitIdent{Name: "Dolt virtual merge base", Email: "virtual@dolt.invalid", Date: datas.CommitDateAt(date)}
	meta, err := datas.NewCommitMetaWithAuthorCommitter(identity, identity, "virtual merge base")
	if err != nil {
		return nil, err
	}
	if _, err = c.vrw.WriteValue(ctx, root.NomsValue()); err != nil {
		return nil, err
	}
	dc, err := datas.NewCommitForValue(ctx, store.ChunkStore(), c.vrw, c.ns, root.NomsValue(), datas.CommitOptions{Parents: parents, Meta: meta})
	if err != nil {
		return nil, err
	}
	if _, err = c.vrw.WriteValue(ctx, dc.NomsValue()); err != nil {
		return nil, err
	}
	return NewCommit(ctx, c.vrw, c.ns, dc)
}
