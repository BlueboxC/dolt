// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
)

func TestFulltextOrdinaryFilters(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, script := range queries.FulltextOrdinaryFilterTests {
			t.Run(fmt.Sprintf("prepared=%v/%s", prepared, script.Name), func(t *testing.T) {
				h := newDoltHarness(t)
				defer h.Close()
				if prepared {
					enginetest.TestScriptPrepared(t, h, script)
				} else {
					enginetest.TestScript(t, h, script)
				}
			})
		}
	}
}
