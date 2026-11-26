// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package enginetest

import (
	"fmt"
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/types"
	"github.com/stretchr/testify/require"
)

func TestFulltextOrdinaryFilters(t *testing.T) {
	runForkFulltextLexicalAliases(t)
	runForkOriginalExpressionScripts(t)
	runForkOriginalFulltextScripts(t)
	runForkOriginalFulltextNoPanic(t)
	runForkOriginalUnionScripts(t)
	runForkInsertIgnoreOmittedDefaultControl(t)
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

// Select exact retained scripts; leave their SQL, expected rows and diagnostics unchanged.
func runForkOriginalExpressionScripts(t *testing.T) {
	t.Helper()
	wanted := map[string]bool{
		"fork subquery unnesting preserves other negations":                true,
		"fork join predicates require every referenced relation":           true,
		"fork NULL extended relation remains in predicate eligibility":     true,
		"fork indexed comparisons retain the original value":               true,
		"fork explicit collation cannot reuse incompatible index ordering": true,
		"fork unique NULL join keys obey comparison semantics":             true,
		"fork mixed IN subquery uses SQL comparison":                       true,
		"fork negated scalar subquery retains cardinality and NULL":        true,
		"fork range heap preserves input filters":                          true,
		"fork fulltext returns each keyed document once":                   true,
		"fork fulltext is not an ordered column index":                     true,
		"fork decimal comparisons retain both scales":                      true,
		"fork LIKE escape rejects row-dependent expressions":               true,
		"coalesce tests": true,
	}
	selected := make([]queries.ScriptTest, 0, len(wanted))
	seen := make(map[string]bool, len(wanted))
	for _, script := range queries.ExpressionsScriptTests {
		if !wanted[script.Name] {
			continue
		}
		if seen[script.Name] {
			t.Fatalf("duplicate retained script %q", script.Name)
		}
		seen[script.Name] = true
		selected = append(selected, script)
	}
	if len(selected) != len(wanted) {
		t.Fatalf("missing retained scripts: found %d, want %d", len(selected), len(wanted))
	}
	for _, prepared := range []bool{false, true} {
		mode := "normal"
		if prepared {
			mode = "prepared"
		}
		for _, script := range selected {
			t.Run("original/"+mode+"/"+script.Name, func(t *testing.T) {
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

// Select exact retained scripts; leave their SQL, expected rows and diagnostics unchanged.
func runForkOriginalFulltextScripts(t *testing.T) {
	t.Helper()
	wanted := map[string]bool{
		"MATCH AGAINST over a derived table column":                            true,
		"MATCH AGAINST in a JOIN ON clause":                                    true,
		"MATCH AGAINST in a JOIN ON clause over a derived table":               true,
		"MATCH AGAINST in a JOIN ON clause over renamed derived table columns": true,
	}
	selected := make([]queries.ScriptTest, 0, len(wanted))
	seen := make(map[string]bool, len(wanted))
	for _, script := range queries.FulltextTests {
		if !wanted[script.Name] {
			continue
		}
		if seen[script.Name] {
			t.Fatalf("duplicate retained script %q", script.Name)
		}
		seen[script.Name] = true
		selected = append(selected, script)
	}
	if len(selected) != len(wanted) {
		t.Fatalf("missing retained scripts: found %d, want %d", len(selected), len(wanted))
	}
	for _, prepared := range []bool{false, true} {
		mode := "normal"
		if prepared {
			mode = "prepared"
		}
		for _, script := range selected {
			t.Run("original/"+mode+"/"+script.Name, func(t *testing.T) {
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

// Preserve original UNION names,types,rows and diagnostics without copying SQL.
func runForkOriginalUnionScripts(t *testing.T) {
	t.Helper()
	wanted := map[string]bool{
		"NULL union branches preserve authored column identity and numeric type": true,
		"UNION ALL with BLOB columns":                                            true,
	}
	selected := make([]queries.ScriptTest, 0, len(wanted))
	seen := make(map[string]bool, len(wanted))
	for _, script := range queries.SetOperationsScriptTests {
		if !wanted[script.Name] {
			continue
		}
		if seen[script.Name] {
			t.Fatalf("duplicate retained UNION script %q", script.Name)
		}
		seen[script.Name] = true
		selected = append(selected, script)
	}
	if len(selected) != len(wanted) {
		t.Fatalf("missing retained UNION scripts: found %d, want %d", len(selected), len(wanted))
	}
	for _, prepared := range []bool{false, true} {
		mode := "normal"
		if prepared {
			mode = "prepared"
		}
		for _, script := range selected {
			t.Run("original/"+mode+"/"+script.Name, func(t *testing.T) {
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

// Run the original MATCH query in this callback's goroutine so a planning panic
// becomes an assertion failure. Restore the complete normal/prepared script call
// before final qualification; this diagnostic does not replace those controls.
func runForkOriginalFulltextNoPanic(t *testing.T) {
	t.Helper()
	wanted := map[string]bool{
		"MATCH AGAINST over a derived table column":                            true,
		"MATCH AGAINST in a JOIN ON clause":                                    true,
		"MATCH AGAINST in a JOIN ON clause over a derived table":               true,
		"MATCH AGAINST in a JOIN ON clause over renamed derived table columns": true,
	}
	seen := make(map[string]bool, len(wanted))
	for _, script := range queries.FulltextTests {
		if !wanted[script.Name] {
			continue
		}
		require.False(t, seen[script.Name], "duplicate original MATCH script %q", script.Name)
		seen[script.Name] = true
		t.Run("original/direct-no-panic/"+script.Name, func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			e := enginetest.MustNewEngine(t, h)
			defer e.Close()
			ctx := enginetest.NewContext(h)
			for _, statement := range script.SetUpScript {
				queryCtx := ctx.WithQuery(statement)
				_, iter, _, err := e.QueryWithBindings(queryCtx, statement, nil, nil, nil)
				require.NoError(t, err, "original MATCH setup: %s", statement)
				_, err = sql.RowIterToRows(queryCtx, iter)
				require.NoError(t, err, "original MATCH setup: %s", statement)
			}
			require.Len(t, script.Assertions, 1, "expected exactly the preserved original MATCH assertion")
			assertion := script.Assertions[0]
			require.Nil(t, assertion.ExpectedErr)
			require.Empty(t, assertion.ExpectedErrStr)
			queryCtx := ctx.WithQuery(assertion.Query)
			var rows []sql.Row
			require.NotPanics(t, func() {
				_, iter, _, err := e.QueryWithBindings(queryCtx, assertion.Query, nil, assertion.Bindings, nil)
				require.NoError(t, err, "original MATCH query: %s", assertion.Query)
				rows, err = sql.RowIterToRows(queryCtx, iter)
				require.NoError(t, err, "original MATCH query: %s", assertion.Query)
			}, "original MATCH query must plan and iterate without panicking: %s", assertion.Query)
			require.Len(t, rows, len(assertion.Expected), "original MATCH result cardinality")
		})
	}
	require.Len(t, seen, len(wanted), "missing preserved original MATCH scripts")
}

// NEW control for the retained INSERT IGNORE implicit-default contract. The SQL
// below is a new regression fixture, not one of the selected original scripts.
func runForkInsertIgnoreOmittedDefaultControl(t *testing.T) {
	t.Helper()
	script := queries.ScriptTest{
		Name: "new INSERT IGNORE omitted required column preserves generated values",
		SetUpScript: []string{
			"CREATE TABLE r8_ignore_default (pk INT PRIMARY KEY, a INT NOT NULL, g INT AS (a+100) STORED)",
		},
		Assertions: []queries.ScriptTestAssertion{
			{
				Query:    "INSERT IGNORE INTO r8_ignore_default(pk,a) VALUES (3,9)",
				Expected: []sql.Row{{types.OkResult{RowsAffected: 1}}},
			},
			{
				Query:    "SELECT pk,a,g FROM r8_ignore_default WHERE pk=3",
				Expected: []sql.Row{{3, 9, 109}},
			},
			{
				Query:                           "INSERT IGNORE INTO r8_ignore_default(pk) VALUES (1)",
				Expected:                        []sql.Row{{types.OkResult{RowsAffected: 1}}},
				ExpectedWarning:                 1364,
				ExpectedWarningsCount:           1,
				ExpectedWarningMessageSubstring: "Field 'a' doesn't have a default value",
			},
			{
				Query:    "SELECT pk,a,g FROM r8_ignore_default WHERE pk=1",
				Expected: []sql.Row{{1, 0, 100}},
			},
			{
				Query:                           "INSERT IGNORE INTO r8_ignore_default(pk) VALUES (2)",
				Expected:                        []sql.Row{{types.OkResult{RowsAffected: 1}}},
				ExpectedWarning:                 1364,
				ExpectedWarningsCount:           1,
				ExpectedWarningMessageSubstring: "Field 'a' doesn't have a default value",
			},
			{
				Query:    "SELECT pk,a,g FROM r8_ignore_default ORDER BY pk",
				Expected: []sql.Row{{1, 0, 100}, {2, 0, 100}, {3, 9, 109}},
			},
		},
	}
	for _, prepared := range []bool{false, true} {
		t.Run(fmt.Sprintf("new/insert-ignore-default/prepared=%v", prepared), func(t *testing.T) {
			h := newDoltHarness(t)
			defer h.Close()
			e := enginetest.MustNewEngine(t, h)
			defer e.Close()
			ctx := enginetest.NewContext(h)
			for _, statement := range script.SetUpScript {
				enginetest.RunQueryWithContext(t, e, h, ctx.WithQuery(statement), statement)
			}
			for _, assertion := range script.Assertions {
				t.Run(assertion.Query, func(t *testing.T) {
					queryCtx := ctx.WithQuery(assertion.Query)
					queryCtx.ClearWarnings()
					if prepared {
						enginetest.TestPreparedQueryWithContext(t, queryCtx, e, h, assertion.Query, assertion.Expected, nil, nil, false)
					} else {
						enginetest.TestQueryWithContext(t, queryCtx, e, h, assertion.Query, assertion.Expected, nil, nil, nil)
					}
					warnings := queryCtx.Warnings()
					require.Len(t, warnings, assertion.ExpectedWarningsCount, "warning count for this execution")
					for _, warning := range warnings {
						require.Equal(t, assertion.ExpectedWarning, warning.Code, "warning code for this execution")
						require.Contains(t, warning.Message, assertion.ExpectedWarningMessageSubstring, "missing-default diagnostic")
					}
				})
			}
		})
	}
}

// A subquery may reuse a visible alias without binding the outer MATCH to it.
func runForkFulltextLexicalAliases(t *testing.T) {
	t.Helper()
	cases := []struct {
		name  string
		query string
	}{
		{"shadowed_inner_d", "SELECT d.id FROM outer_docs AS d CROSS JOIN (SELECT body FROM (SELECT body FROM other_docs) AS d) AS x WHERE MATCH(d.body) AGAINST ('alpha')"},
		{"distinct_inner_alias", "SELECT d.id FROM outer_docs AS d CROSS JOIN (SELECT body FROM (SELECT body FROM other_docs) AS y) AS x WHERE MATCH(d.body) AGAINST ('alpha')"},
		{"without_nested_shadow", "SELECT d.id FROM outer_docs AS d CROSS JOIN other_docs AS x WHERE MATCH(d.body) AGAINST ('alpha')"},
		{"ordinary_shadowed_inner_d", "SELECT d.id FROM outer_docs AS d CROSS JOIN (SELECT d.body FROM other_docs AS d) AS x WHERE MATCH(d.body) AGAINST ('alpha')"},
		{"ordinary_distinct_inner_alias", "SELECT d.id FROM outer_docs AS d CROSS JOIN (SELECT y.body FROM other_docs AS y) AS x WHERE MATCH(d.body) AGAINST ('alpha')"},
		{"ordinary_shadow_before_outer", "SELECT d.id FROM (SELECT d.body FROM other_docs AS d) AS x CROSS JOIN outer_docs AS d WHERE MATCH(d.body) AGAINST ('alpha')"},
	}
	t.Run("lexical_alias", func(t *testing.T) {
		for _, prepared := range []bool{false, true} {
			t.Run(fmt.Sprintf("prepared=%v", prepared), func(t *testing.T) {
				h := newDoltHarness(t)
				defer h.Close()
				e := enginetest.MustNewEngine(t, h)
				defer e.Close()
				ctx := enginetest.NewContext(h)
				for _, statement := range []string{
					"CREATE TABLE outer_docs (id INT PRIMARY KEY, body TEXT, FULLTEXT INDEX ft (body))",
					"INSERT INTO outer_docs VALUES (1, 'alpha')",
					"CREATE TABLE other_docs (body TEXT)",
					"INSERT INTO other_docs VALUES ('other')",
				} {
					enginetest.RunQueryWithContext(t, e, h, ctx.WithQuery(statement), statement)
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						queryCtx := ctx.WithQuery(tc.query)
						expected := []sql.Row{{int32(1)}}
						if prepared {
							enginetest.TestPreparedQueryWithContext(t, queryCtx, e, h, tc.query, expected, nil, nil, false)
						} else {
							enginetest.TestQueryWithContext(t, queryCtx, e, h, tc.query, expected, nil, nil, nil)
						}
					})
				}
			})
		}
	})
}
