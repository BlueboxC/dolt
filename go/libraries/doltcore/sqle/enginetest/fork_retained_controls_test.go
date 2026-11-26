// Retained authored assertions from the exact qualified fork; normal/prepared runners below.
package enginetest

import (
	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"testing"
)

var ForkRetainedMergeMultipleBasesScripts = []queries.ScriptTest{
	{
		// x1 adds ts with a time default, and y1 adds row 2, so merging them gives row 2 a default ts. The
		// virtual merge base takes its own date for it, the later of x1's and y1's commit dates, in UTC, so that
		// previewing later, in any time zone, or rebuilding it, gives the same value.
		Name:        "merge bases that give a row a time default: merge y into x",
		SetUpScript: timeDefaultMergeBases().Script(),
		Assertions: []queries.ScriptTestAssertion{
			{Query: "SET time_zone = '-07:00';", SkipResultsCheck: true},
			{
				Query:    "SELECT base_ts = (SELECT MAX(date) FROM dolt_log('--all') WHERE message IN ('x1', 'y1')) FROM dolt_preview_merge_conflicts('x', 'y', 't') WHERE base_id = 2;",
				Expected: []sql.Row{{true}},
			},
			{Query: "SET time_zone = '+00:00';", SkipResultsCheck: true},
			{
				Query:    "SELECT base_ts = (SELECT MAX(date) FROM dolt_log('--all') WHERE message IN ('x1', 'y1')) FROM dolt_preview_merge_conflicts('x', 'y', 't') WHERE base_id = 2;",
				Expected: []sql.Row{{true}},
			},
			{Query: "SET time_zone = '-07:00';", SkipResultsCheck: true},
			{
				Query:          "CALL DOLT_MERGE('y', '-m', 'merge y');",
				ExpectedErrStr: "best common ancestors conflict; virtual merge base requires explicit resolution",
			},
			{
				Query:    "SELECT @@time_zone;",
				Expected: []sql.Row{{"-07:00"}},
			},
		},
	},
}

func TestForkRetainedControls(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		for _, script := range ForkRetainedMergeMultipleBasesScripts {
			h := newDoltHarness(t)
			if prepared {
				enginetest.TestScriptPrepared(t, h, script)
			} else {
				enginetest.TestScript(t, h, script)
			}
			h.Close()
		}
	}
}
