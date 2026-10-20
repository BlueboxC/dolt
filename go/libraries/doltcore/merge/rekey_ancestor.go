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
	"github.com/dolthub/dolt/go/libraries/doltcore/table/editor/creation"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/go-mysql-server/sql"
	"io"

	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb/durable"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/store/prolly"
	"github.com/dolthub/dolt/go/store/val"
)

// rekeyAncestor copies the ancestor's row layout when both branches made the
// same key change. Unsupported layouts retain SchemaMerge's existing refusal.
func (tm *TableMerger) rekeyAncestor(ctx *sql.Context) error {
	parent, ok := tm.ancestorSrc.(*doltdb.Commit)
	if !ok {
		return nil
	}
	if tm.leftTbl == nil || tm.rightTbl == nil || tm.ancTbl == nil ||
		schema.IsKeyless(tm.leftSch) || schema.IsKeyless(tm.ancSch) ||
		schema.ComparePrimaryKeys(tm.leftSch, tm.rightSch) != schema.PrimaryKeysSameTags ||
		schema.ComparePrimaryKeys(tm.leftSch, tm.ancSch) == schema.PrimaryKeysSameTags {
		return nil
	}
	for _, idx := range tm.ancSch.Indexes().AllIndexes() {
		if idx.IsFullText() || idx.IsSpatial() || idx.IsVector() || idx.Predicate() != "" {
			return nil
		}
	}
	artifacts, err := tm.ancTbl.GetArtifacts(ctx)
	if err != nil {
		return err
	}
	count, err := artifacts.Count()
	if err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	cols := append([]schema.Column(nil), tm.ancSch.GetAllCols().GetColumns()...)
	for _, c := range cols {
		if c.Virtual {
			return nil
		}
	}
	pk := tm.leftSch.GetPKCols()
	ordinals := make([]int, pk.Size())
	for i, c := range pk.GetColumns() {
		old, ok := tm.ancSch.GetAllCols().GetByTag(c.Tag)
		if !ok || !old.TypeInfo.Equals(c.TypeInfo) {
			return nil
		}
		ordinals[i] = tm.ancSch.GetAllCols().TagToIdx[c.Tag]
	}
	for i := range cols {
		_, cols[i].IsPartOfPK = pk.GetByTag(cols[i].Tag)
		if cols[i].IsPartOfPK {
			hasNotNull := false
			for _, c := range cols[i].Constraints {
				hasNotNull = hasNotNull || c.GetConstraintType() == schema.NotNullConstraintType
			}
			if !hasNotNull {
				cols[i].Constraints = append(append([]schema.ColConstraint(nil), cols[i].Constraints...), schema.NotNullConstraint{})
			}
		}
	}
	target, err := schema.NewSchema(schema.NewColCollection(cols...), ordinals, tm.ancSch.GetCollation(), nil, tm.ancSch.Checks())
	if err != nil {
		return err
	}
	target.Indexes().AddIndex(tm.ancSch.Indexes().AllIndexes()...)
	original, err := rowsFromTable(ctx, tm.ancTbl)
	if err != nil {
		return err
	}
	kd, vd := target.GetMapDescriptors(tm.ns)
	oldKD, oldVD := tm.ancSch.GetMapDescriptors(tm.ns)
	type field struct {
		key     bool
		ordinal int
	}
	fields := make(map[uint64]field, len(cols))
	for i, c := range tm.ancSch.GetPKCols().GetColumns() {
		fields[c.Tag] = field{true, i}
	}
	for i, c := range tm.ancSch.GetNonPKCols().GetColumns() {
		if !c.Virtual {
			fields[c.Tag] = field{false, i}
		}
	}
	empty, err := prolly.NewMapFromTuples(ctx, tm.ns, kd, vd)
	if err != nil {
		return err
	}
	mut := empty.Mutate()
	iter, err := original.IterAll(ctx)
	if err != nil {
		return err
	}
	keyBuilder, valueBuilder := val.NewTupleBuilder(kd, tm.ns), val.NewTupleBuilder(vd, tm.ns)
	for {
		k, v, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		copyFields := func(columns []schema.Column, b *val.TupleBuilder, required bool) error {
			for i, c := range columns {
				src := fields[c.Tag]
				tuple := v
				if src.key {
					tuple = k
				}
				raw := tuple.GetField(src.ordinal)
				if required && raw == nil {
					return fmt.Errorf("cannot re-key ancestor of %s: NULL in the new primary key", tm.name)
				}
				desc := oldVD
				if src.key {
					desc = oldKD
				}
				if desc.Types[src.ordinal].Enc == b.Desc.Types[i].Enc {
					b.PutRaw(i, raw)
				} else {
					value, err := tree.GetField(ctx, desc, src.ordinal, tuple, tm.ns)
					if err != nil {
						return err
					}
					if err = tree.PutField(ctx, tm.ns, b, i, value); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err = copyFields(target.GetPKCols().GetColumns(), keyBuilder, true); err != nil {
			return err
		}
		if err = copyFields(target.GetNonPKCols().GetColumns(), valueBuilder, false); err != nil {
			return err
		}
		key, err := keyBuilder.Build(ctx, original.Pool())
		if err != nil {
			return err
		}
		value, err := valueBuilder.Build(ctx, original.Pool())
		if err != nil {
			return err
		}
		exists, err := mut.Has(ctx, key)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("cannot re-key ancestor of %s: rows collide under the new primary key", tm.name)
		}
		if err = mut.Put(ctx, key, value); err != nil {
			return err
		}
	}
	rebuilt, err := mut.Map(ctx)
	if err != nil {
		return err
	}
	table, err := tm.ancTbl.UpdateSchema(ctx, target)
	if err != nil {
		return err
	}
	table, err = table.UpdateRows(ctx, durable.IndexFromProllyMap(rebuilt))
	if err != nil {
		return err
	}
	indexes, err := durable.NewIndexSet(ctx, tm.vrw, tm.ns)
	if err != nil {
		return err
	}
	for _, idx := range target.Indexes().AllIndexes() {
		rebuiltIndex, err := creation.BuildSecondaryProllyIndex(ctx, tm.vrw, tm.ns, target, tm.name.Name, idx, rebuilt, nil)
		if err != nil {
			return err
		}
		indexes, err = indexes.PutIndex(ctx, idx.Name(), rebuiltIndex)
		if err != nil {
			return err
		}
	}
	table, err = table.SetIndexSet(ctx, indexes)
	if err != nil {
		return err
	}
	root, err := tm.ancestorSrc.ResolveRootValue(ctx)
	if err != nil {
		return err
	}
	root, err = root.PutTable(ctx, tm.name, table)
	if err != nil {
		return err
	}
	ancestor, err := parent.NewVirtualAncestor(ctx, root, nil)
	if err != nil {
		return err
	}
	tm.ancTbl, tm.ancSch, tm.ancestorSrc = table, target, ancestor
	return nil
}
