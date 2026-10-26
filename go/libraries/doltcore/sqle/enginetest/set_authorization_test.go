// Copyright 2026 Dolthub, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package enginetest

import (
	"context"
	"database/sql"
	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/dolt/go/libraries/doltcore/servercfg"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestDoltSetGlobalAuthorization(t *testing.T) {
	dEnv, controller, cfg := startServer(t, true, "127.0.0.1", "")
	defer func() { controller.Stop(); require.NoError(t, controller.WaitForStop()); dEnv.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	connect := func(user string) *sql.Conn {
		dsn, err := mysql.ParseDSN(servercfg.ConnectionString(cfg, "dolt"))
		require.NoError(t, err)
		dsn.User = user
		db, err := sql.Open("mysql", dsn.FormatDSN())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, db.Close()) })
		connection, err := db.Conn(ctx)
		require.NoError(t, err)
		return connection
	}
	root := connect("root")
	defer root.Close()
	exec := func(connection *sql.Conn, query string) {
		t.Helper()
		_, err := connection.ExecContext(ctx, query)
		require.NoError(t, err, query)
	}
	denied := func(err error) {
		t.Helper()
		var mysqlErr *mysql.MySQLError
		require.ErrorAs(t, err, &mysqlErr)
		require.Equal(t, uint16(1142), mysqlErr.Number)
	}
	global := func(expected int) {
		t.Helper()
		var actual int
		require.NoError(t, root.QueryRowContext(ctx, "SELECT @@GLOBAL.max_connections").Scan(&actual))
		require.Equal(t, expected, actual)
	}
	localConfig, ok := dEnv.Config.GetConfig(env.LocalConfig)
	require.True(t, ok)
	persisted := func(expected string) {
		t.Helper()
		actual, err := localConfig.GetString(env.SqlServerGlobalsPrefix + ".max_connections")
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	exec(root, "CREATE USER 'variable_reader'@'%' IDENTIFIED BY ''")
	exec(root, "GRANT SELECT ON dolt.* TO 'variable_reader'@'%'")
	exec(root, "SET PERSIST max_connections = 100")
	reader := connect("variable_reader")
	defer reader.Close()
	exec(reader, "SET @set_marker = 1")
	for _, query := range []string{
		"SET GLOBAL max_connections = 101", "SET @@gLoBaL.max_connections = 101",
		"SET PERSIST max_connections = 101", "SET @@PERSIST.max_connections = 101",
		"SET PERSIST_ONLY max_connections = 101", "SET @@PERSIST_ONLY.max_connections = 101",
		"SET GLOBAL read_only = 1", "SET @set_marker = 2, @@GLOBAL.max_connections = 101",
	} {
		_, err := reader.ExecContext(ctx, query)
		denied(err)
		global(100)
		persisted("100")
	}
	var marker int
	require.NoError(t, reader.QueryRowContext(ctx, "SELECT @set_marker").Scan(&marker))
	require.Equal(t, 1, marker)
	exec(reader, "SET SESSION sql_select_limit = 20")
	exec(root, "GRANT SYSTEM_VARIABLES_ADMIN ON *.* TO 'variable_reader'@'%'")
	prepared, err := reader.PrepareContext(ctx, "SET GLOBAL max_connections = ?")
	require.NoError(t, err)
	defer prepared.Close()
	_, err = prepared.ExecContext(ctx, 102)
	require.NoError(t, err)
	global(102)
	exec(root, "REVOKE SYSTEM_VARIABLES_ADMIN ON *.* FROM 'variable_reader'@'%'")
	_, err = prepared.ExecContext(ctx, 103)
	denied(err)
	global(102)
	persisted("100")
	exec(root, "GRANT SYSTEM_VARIABLES_ADMIN ON *.* TO 'variable_reader'@'%'")
	exec(reader, "SET PERSIST_ONLY max_connections = 104")
	global(102)
	persisted("104")
	exec(reader, "SET PERSIST max_connections = 105")
	global(105)
	persisted("105")
}
