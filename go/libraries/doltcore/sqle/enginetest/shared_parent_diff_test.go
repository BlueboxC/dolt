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

package enginetest

import (
	"testing"

	"github.com/dolthub/go-mysql-server/enginetest"
	"github.com/dolthub/go-mysql-server/enginetest/queries"
	"github.com/dolthub/go-mysql-server/enginetest/scriptgen/setup"
	"github.com/dolthub/go-mysql-server/sql"
)

func TestSharedParentDiff(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		name := "normal"
		if prepared {
			name = "prepared"
		}
		t.Run(name, func(t *testing.T) {
			h := newDoltEnginetestHarness(t)
			defer h.Close()
			h.Setup(setup.MydbData)
			script := queries.ScriptTest{Name: "forks retain every child-parent diff", SetUpScript: []string{
				"create table t (pk int primary key, v varchar(10))",
				"insert into t values (1,'a'),(2,'b')",
				"call dolt_commit('-Am','c1')",
				"call dolt_branch('feat')", "call dolt_checkout('feat')",
				"update t set v='b2' where pk=2", "call dolt_commit('-am','c2')",
				"call dolt_checkout('main')", "update t set v='a2' where pk=1",
				"call dolt_commit('-am','c3')", "call dolt_merge('--no-ff','feat','-m','merge')",
			}, Assertions: []queries.ScriptTestAssertion{
				{Query: "select to_pk,to_v from dolt_diff_t where to_commit in (hashof('feat'),hashof('main~1')) order by to_pk", Expected: []sql.Row{{1, "a2"}, {2, "b2"}}},
				{Query: "select to_pk,to_v from dolt_diff_t where to_commit in (hashof('main~1'),hashof('feat'),hashof('feat')) order by to_pk", Expected: []sql.Row{{1, "a2"}, {2, "b2"}}},
				{Query: "select to_pk,to_v from dolt_diff_t where to_commit=hashof('feat')", Expected: []sql.Row{{2, "b2"}}},
				{Query: "select to_pk,to_v from dolt_diff_t where to_commit=hashof('main~1')", Expected: []sql.Row{{1, "a2"}}},
				{Query: "select count(*) from dolt_diff_t", Expected: []sql.Row{{6}}},
				{Query: "select to_pk,to_v from dolt_diff_t where from_commit=hashof('main~2') order by to_pk", Expected: []sql.Row{{1, "a2"}, {2, "b2"}}},
				{Query: "select to_pk,to_v from dolt_diff_t where from_commit in (hashof('main~2'),hashof('feat'),hashof('main~1')) order by to_pk,to_v", Expected: []sql.Row{{1, "a2"}, {1, "a2"}, {2, "b2"}, {2, "b2"}}},
				{Query: "select to_pk,to_v from dolt_diff_t where from_commit=hashof('main')", Expected: []sql.Row{}},

				{Query: "select to_pk,to_v from dolt_diff_t where to_commit in (hashof('main'),hashof('feat'),hashof('main~1')) order by to_pk,to_v", Expected: []sql.Row{{1, "a2"}, {1, "a2"}, {2, "b2"}, {2, "b2"}}},
				{Query: "select l.message,d.to_pk,d.to_v from dolt_diff_t d join dolt_log l on l.commit_hash=d.to_commit order by l.message,d.to_pk", Expected: []sql.Row{{"c1", 1, "a"}, {"c1", 2, "b"}, {"c2", 2, "b2"}, {"c3", 1, "a2"}, {"merge", 1, "a2"}, {"merge", 2, "b2"}}},
			}}
			if prepared {
				enginetest.TestScriptPrepared(t, h, script)
			} else {
				enginetest.TestScript(t, h, script)
			}
		})
	}
}
