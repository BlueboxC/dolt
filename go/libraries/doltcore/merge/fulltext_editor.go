// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package merge

import (
	"bytes"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle/index"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/dolthub/dolt/go/store/val"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/expression"
)

// Full-text rebuilds use the same collation-aware primary map as the persisted
// pseudo-tables. A memory.Table editor scans every accumulated row per write.
var _ sql.ForeignKeyEditor = (*fulltextTable)(nil)

func (table *fulltextTable) tuples(ctx *sql.Context, row sql.Row) (val.Tuple, val.Tuple, error) {
	kd, vd := table.edits.Descriptors()
	km, vm := ordinalMappingsFromSchema(table.SqlSch, table.Sch)
	key, err := table.tuple(ctx, row, km, kd)
	if err != nil {
		return nil, nil, err
	}
	value, err := table.tuple(ctx, row, vm, vd)
	return key, value, err
}
func (table *fulltextTable) tuple(ctx *sql.Context, row sql.Row, mapping val.OrdinalMapping, desc *val.TupleDesc) (val.Tuple, error) {
	b := val.NewTupleBuilder(desc, table.rows.NodeStore())
	for to, from := range mapping {
		if err := tree.PutField(ctx, table.rows.NodeStore(), b, to, row[from]); err != nil {
			return nil, err
		}
	}
	return b.Build(ctx, sharePool)
}
func (table *fulltextTable) Insert(ctx *sql.Context, row sql.Row) error {
	k, v, err := table.tuples(ctx, row)
	if err != nil {
		return err
	}
	return table.edits.Put(ctx, k, v)
}
func (table *fulltextTable) Delete(ctx *sql.Context, row sql.Row) error {
	k, _, err := table.tuples(ctx, row)
	if err != nil {
		return err
	}
	return table.edits.Delete(ctx, k)
}
func (table *fulltextTable) Update(ctx *sql.Context, old, new sql.Row) error {
	oldKey, _, err := table.tuples(ctx, old)
	if err != nil {
		return err
	}
	key, value, err := table.tuples(ctx, new)
	if err != nil {
		return err
	}
	if !bytes.Equal(oldKey, key) {
		if err := table.edits.Delete(ctx, oldKey); err != nil {
			return err
		}
	}
	return table.edits.Put(ctx, key, value)
}
func (*fulltextTable) StatementBegin(*sql.Context)          {}
func (*fulltextTable) StatementComplete(*sql.Context) error { return nil }
func (*fulltextTable) Close(*sql.Context) error             { return nil }
func (table *fulltextTable) DiscardChanges(*sql.Context, error) error {
	table.edits = table.rows.Mutate()
	return nil
}

type fulltextPartition struct{ lookup *sql.IndexLookup }

func (fulltextPartition) Key() []byte { return []byte("fulltext") }

type fulltextLookup struct{ *fulltextTable }

func (table *fulltextLookup) LookupPartitions(_ *sql.Context, lookup sql.IndexLookup) (sql.PartitionIter, error) {
	return sql.PartitionsToPartitionIter(fulltextPartition{lookup: &lookup}), nil
}
func (table *fulltextLookup) PartitionRows(ctx *sql.Context, partition sql.Partition) (sql.RowIter, error) {
	lookup := partition.(fulltextPartition).lookup
	ranges, ok := lookup.Ranges.(sql.MySQLRangeCollection)
	if !ok {
		return nil, sql.ErrInvalidType.New(lookup.Ranges)
	}
	// The editor's count lookups specify one complete primary key.
	if len(ranges) == 1 && len(ranges[0]) == len(table.pkOrdinals) {
		row := make(sql.Row, len(table.SqlSch))
		point := true
		for i, column := range ranges[0] {
			lower, lok := column.LowerBound.(sql.Below)
			upper, uok := column.UpperBound.(sql.Above)
			if !lok || !uok {
				point = false
				break
			}
			cmp, err := column.Typ.Compare(ctx, lower.Key, upper.Key)
			if err != nil {
				return nil, err
			}
			if cmp != 0 {
				point = false
				break
			}
			row[table.pkOrdinals[i]] = lower.Key
		}
		if point {
			kd, _ := table.edits.Descriptors()
			km, _ := ordinalMappingsFromSchema(table.SqlSch, table.Sch)
			key, err := table.tuple(ctx, row, km, kd)
			if err != nil {
				return nil, err
			}
			var rows []sql.Row
			err = table.edits.Get(ctx, key, func(k, v val.Tuple) error {
				if v == nil {
					return nil
				}
				found, err := index.BuildRow(ctx, k, v, table.Sch, table.rows.NodeStore())
				if err == nil {
					rows = append(rows, found)
				}
				return err
			})
			return sql.RowsToRowIter(rows...), err
		}
	}
	// Keep non-point lookups correct without optimizing an unused access path.
	fields := make([]sql.Expression, len(table.pkOrdinals))
	for i, ord := range table.pkOrdinals {
		c := table.SqlSch[ord]
		fields[i] = expression.NewGetField(ord, c.Type, c.Name, c.Nullable)
	}
	filter, err := expression.NewRangeFilterExpr(ctx, fields, ranges)
	if err != nil {
		return nil, err
	}
	iter, err := table.fulltextTable.PartitionRows(ctx, partition)
	if err != nil {
		return nil, err
	}
	return &fulltextFilterIter{RowIter: iter, filter: filter}, nil
}

type fulltextFilterIter struct {
	sql.RowIter
	filter sql.Expression
}

func (iter *fulltextFilterIter) Next(ctx *sql.Context) (sql.Row, error) {
	for {
		row, err := iter.RowIter.Next(ctx)
		if err != nil {
			return nil, err
		}
		matches, err := sql.EvaluateCondition(ctx, iter.filter, row)
		if err != nil {
			return nil, err
		}
		if sql.IsTrue(matches) {
			return row, nil
		}
	}
}
