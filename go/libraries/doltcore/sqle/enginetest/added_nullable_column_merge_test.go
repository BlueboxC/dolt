// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"testing"
)

func TestAddedNullableColumnMerge(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			for _, kind := range []string{"backfill", "explicit-null", "one-sided-column", "both-values", "new-row", "non-null-default"} {
				t.Run(fmt.Sprintf("prepared=%v/reverse=%v/%s", prepared, reverse, kind), func(t *testing.T) {
					setup := []string{"CREATE TABLE t(id INT PRIMARY KEY,a VARCHAR(100))"}
					if kind != "new-row" {
						setup = append(setup, "INSERT INTO t VALUES(1,'seed')")
					}
					setup = append(setup, "CALL DOLT_COMMIT('-Am','base')", "CALL DOLT_BRANCH('other')")
					column := "ALTER TABLE t ADD COLUMN c INT"
					if kind == "non-null-default" {
						column += " DEFAULT 7"
					}
					setup = append(setup, column)
					if kind == "new-row" {
						setup = append(setup, "INSERT INTO t VALUES(1,'seed',5)")
					} else {
						setup = append(setup, "UPDATE t SET c=5")
					}
					setup = append(setup, "CALL DOLT_COMMIT('-am','main')", "CALL DOLT_CHECKOUT('other')")
					if kind != "one-sided-column" {
						setup = append(setup, column)
					}
					if kind == "new-row" {
						setup = append(setup, "INSERT INTO t VALUES(1,'seed',NULL)")
					} else {
						setup = append(setup, "UPDATE t SET a='other edit'")
					}
					if kind == "explicit-null" {
						setup = append(setup, "UPDATE t SET c=NULL")
					}
					if kind == "both-values" {
						setup = append(setup, "UPDATE t SET c=6")
					}
					setup = append(setup, "CALL DOLT_COMMIT('-am','other')", "SET autocommit=0", "SET @@dolt_allow_commit_conflicts=1")
					if reverse {
						setup = append(setup, "CALL DOLT_MERGE('main')")
					} else {
						setup = append(setup, "CALL DOLT_CHECKOUT('main')", "CALL DOLT_MERGE('other')")
					}
					conflict := kind == "both-values" || kind == "new-row" || kind == "non-null-default"
					expected := 0
					if conflict {
						expected = 1
					}
					assertions := []queries.ScriptTestAssertion{{Query: "SELECT COUNT(*) FROM dolt_conflicts", Expected: []sql.Row{{expected}}}}
					if !conflict {
						assertions = append(assertions, queries.ScriptTestAssertion{Query: "SELECT * FROM t", Expected: []sql.Row{{1, "other edit", 5}}})
					}
					h := newDoltHarness(t)
					defer h.Close()
					script := queries.ScriptTest{Name: kind, SetUpScript: setup, Assertions: assertions}
					if prepared {
						enginetest.TestScriptPrepared(t, h, script)
					} else {
						enginetest.TestScript(t, h, script)
					}
				})
			}
		}
	}
}
