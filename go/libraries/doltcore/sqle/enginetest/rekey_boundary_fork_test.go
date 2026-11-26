// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"reflect"
	"testing"

	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/libraries/doltcore/schema/typeinfo"
	gmstypes "github.com/dolthub/go-mysql-server/sql/types"
	"github.com/dolthub/vitess/go/vt/proto/query"
)

func r8RekeyBoundarySchema(t *testing.T, info typeinfo.TypeInfo, originalPK bool) schema.Schema {
	t.Helper()
	id, err := schema.NewColumnWithTypeInfo("id", 101, typeinfo.Int64Type, originalPK, "", false, "", schema.NotNullConstraint{})
	if err != nil {
		t.Fatal(err)
	}
	key, err := schema.NewColumnWithTypeInfo("k", 202, info, !originalPK, "", false, "", schema.NotNullConstraint{})
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.SchemaFromCols(schema.NewColCollection(id, key))
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func r8CheckRekeyBoundaries(t *testing.T) {
	plain, err := typeinfo.FromSqlType(gmstypes.Int8)
	if err != nil {
		t.Fatal(err)
	}
	withWidth, err := typeinfo.FromSqlType(gmstypes.MustCreateNumberTypeWithDisplayWidth(query.Type_INT8, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !plain.ToSqlType().Equals(withWidth.ToSqlType()) || plain.Equals(withWidth) {
		t.Fatal("fixture must have equal SQL type and distinct real TypeInfo display width")
	}
	t.Run("same_typeinfo_is_supported", func(t *testing.T) {
		base := r8RekeyBoundarySchema(t, plain, true)
		target := r8RekeyBoundarySchema(t, plain, false)
		got, ok, err := schema.RekeyedSchema(base, target)
		if err != nil || !ok || got == nil {
			t.Fatalf("ordinary identical-TypeInfo rekey failed: supported=%v err=%v", ok, err)
		}
		if !reflect.DeepEqual(got.GetPKCols().Tags, []uint64{202}) {
			t.Fatalf("wrong reconstructed primary key: %v", got.GetPKCols().Tags)
		}
	})
	t.Run("same_sql_type_different_typeinfo_is_refused", func(t *testing.T) {
		base := r8RekeyBoundarySchema(t, plain, true)
		target := r8RekeyBoundarySchema(t, withWidth, false)
		got, ok, err := schema.RekeyedSchema(base, target)
		if err != nil || ok || got != nil {
			t.Fatalf("changed TypeInfo must refuse rekey: supported=%v schema=%v err=%v", ok, got, err)
		}
	})
	t.Run("partial_index_is_refused", func(t *testing.T) {
		base := r8RekeyBoundarySchema(t, plain, true)
		target := r8RekeyBoundarySchema(t, plain, false)
		index, err := base.Indexes().AddIndexByColTags("partial_k", []uint64{202}, nil, schema.IndexProperties{Predicate: "k > 0"})
		if err != nil {
			t.Fatal(err)
		}
		if index.Predicate() != "k > 0" {
			t.Fatal("fixture lost real partial-index predicate")
		}
		got, ok, err := schema.RekeyedSchema(base, target)
		if err != nil || ok || got != nil {
			t.Fatalf("partial-index rekey must refuse: supported=%v schema=%v err=%v", ok, got, err)
		}
	})
}
