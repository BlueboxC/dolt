// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
)

func TestForkSQLTypeAndJoinCorrections(t *testing.T) {
	scripts := append([]queries.ScriptTest(nil), queries.GreatestLeastTypeForkTests...)
	for _, script := range queries.JoinScriptTests {
		if script.Name == "join reordering does not drop filters of large cross join" {
			scripts = append(scripts, script)
		}
	}
	if len(scripts) != 2 {
		t.Fatal("missing upstream JOIN regression")
	}
	for _, script := range queries.NumericScriptTests {
		if strings.HasPrefix(script.Name, "Greatest and least preserve exact decimal") || strings.HasPrefix(strings.ToLower(script.Name), "coalesce") {
			scripts = append(scripts, script)
		}
	}
	for _, wire := range []bool{false, true} {
		for _, prepared := range []bool{false, true} {
			t.Run(fmt.Sprintf("wire=%v/prepared=%v", wire, prepared), func(t *testing.T) {
				for _, script := range scripts {
					if wire && prepared {
						// The harness transmits decimal literals as VARCHAR parameters.
						// Restore their SQL type so this tests decimals, not string ordering.
						script.Assertions = append([]queries.ScriptTestAssertion(nil), script.Assertions...)
						for i, a := range script.Assertions {
							if strings.HasPrefix(strings.ToLower(script.Name), "coalesce") {
								script.Assertions[i].Query = strings.ReplaceAll(a.Query, "1.5", "CAST('1.5' AS DECIMAL(2,1))")
							}
							for _, fn := range []string{"GREATEST", "LEAST"} {
								if strings.HasPrefix(a.Query, "SELECT "+fn+"(1.0,") {
									script.Assertions[i].Expected = []sql.Row{{"1.000000000000000000000000000000"}}
									script.Assertions[i].Query = "SELECT " + fn + "(CAST('1.0' AS DECIMAL(31,30)),CAST('1.000000000000000000000000000000' AS DECIMAL(31,30)))"
								}
							}
						}
					}
					t.Run(script.Name, func(t *testing.T) {
						var h *DoltHarness
						if wire {
							h = newDoltServerTestHarness(t)
						} else {
							h = newDoltHarness(t)
						}
						defer h.Close()
						if prepared {
							enginetest.TestScriptPrepared(t, h, script)
						} else {
							enginetest.TestScript(t, h, script)
						}
					})
				}
			})
		}
	}
}
