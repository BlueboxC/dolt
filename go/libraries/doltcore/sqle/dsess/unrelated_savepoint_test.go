// Copyright 2021 Dolthub, Inc.
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
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/go-mysql-server/sql"
	sqltypes "github.com/dolthub/go-mysql-server/sql/types"
	"testing"
)

type unrelatedSavepointDatabase struct{ SqlDatabase }

func (unrelatedSavepointDatabase) Name() string { return "unrelated_dropped" }
func (unrelatedSavepointDatabase) DbData() env.DbData[*sql.Context] {
	return env.DbData[*sql.Context]{Ddb: &doltdb.DoltDB{}}
}

type unrelatedSavepointProvider struct{ emptyRevisionDatabaseProvider }

func (unrelatedSavepointProvider) DoltDatabases() []SqlDatabase {
	return []SqlDatabase{unrelatedSavepointDatabase{}}
}

func TestSavepointIgnoresUnrelatedDatabase(t *testing.T) {
	if _, _, ok := sql.SystemVariables.GetGlobal(TransactionsDisabledSysVar); !ok {
		sql.SystemVariables.AddSystemVariables([]sql.SystemVariable{&sql.MysqlSystemVariable{Name: TransactionsDisabledSysVar, Scope: sql.GetMysqlScope(sql.SystemVariableScope_Session), Dynamic: true, Type: sqltypes.NewSystemBoolType(TransactionsDisabledSysVar), Default: int8(0)}})
	}
	s := DefaultSession(unrelatedSavepointProvider{}, nil)
	ctx := sql.NewContext(context.Background(), sql.WithSession(s))
	tx := &DoltTransaction{}
	if err := s.CreateSavepoint(ctx, tx, "kept"); err != nil {
		t.Fatalf("unrelated disappeared database broke session savepoint: %v", err)
	}
	if len(s.dbStates) != 0 {
		t.Fatal("savepoint loaded an unrelated database")
	}
	if err := s.RollbackToSavepoint(ctx, tx, "kept"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseSavepoint(ctx, tx, "kept"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReleaseSavepoint(ctx, tx, "kept"); !sql.ErrSavepointDoesNotExist.Is(err) {
		t.Fatalf("released savepoint remained available: %v", err)
	}
}
