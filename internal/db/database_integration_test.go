//go:build integration

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

type integrationDatabaseConfig struct {
	host     string
	port     string
	user     string
	password string
	database string
}

func integrationConfig(t *testing.T) integrationDatabaseConfig {
	t.Helper()
	config := integrationDatabaseConfig{
		host:     os.Getenv("MAXIM_TEST_DB_HOST"),
		port:     os.Getenv("MAXIM_TEST_DB_PORT"),
		user:     os.Getenv("MAXIM_TEST_DB_USER"),
		password: os.Getenv("MAXIM_TEST_DB_PASSWORD"),
		database: os.Getenv("MAXIM_TEST_DB_NAME"),
	}
	if config.host == "" || config.port == "" || config.user == "" || config.database == "" {
		t.Fatal("integration database is not configured; set the MAXIM_TEST_DB_* environment variables")
	}
	return config
}

func openIntegrationDatabase(t *testing.T) (*sql.DB, integrationDatabaseConfig) {
	t.Helper()
	config := integrationConfig(t)
	database, err := ConnectPostgres(ConnectionOptions{
		Host: config.host, Port: config.port, User: config.user,
		Password: config.password, Database: config.database, SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("connect to integration database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database, config
}

func uniqueDatabaseObject(prefix string) string {
	return fmt.Sprintf("maxim_it_%s_%d", prefix, time.Now().UnixNano())
}

func TestIntegrationConnectAndListDatabases(t *testing.T) {
	database, config := openIntegrationDatabase(t)

	databases, err := ListDatabases(database)
	if err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if !slices.Contains(databases, config.database) {
		t.Fatalf("connected database %q missing from database list: %v", config.database, databases)
	}

	_, err = ConnectPostgres(ConnectionOptions{
		Host: config.host, Port: config.port, User: config.user,
		Password: config.password + "-wrong", Database: config.database, SSLMode: "disable",
	})
	if err == nil {
		t.Fatal("connection unexpectedly succeeded with an invalid password")
	}
}

func TestIntegrationBrowseHonorsCancellation(t *testing.T) {
	database, _ := openIntegrationDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := BrowseTableContext(ctx, database, "table_that_is_never_queried", TableBrowseRequest{Limit: 10}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled browse returned %v, want context.Canceled", err)
	}
	if _, err := GetTablesContext(ctx, database); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled table discovery returned %v, want context.Canceled", err)
	}
}

func TestIntegrationSchemaDiscoveryAndPagination(t *testing.T) {
	database, _ := openIntegrationDatabase(t)
	tableName := uniqueDatabaseObject("records")
	quotedTable := pq.QuoteIdentifier(tableName)
	t.Cleanup(func() { _, _ = database.Exec("DROP TABLE IF EXISTS " + quotedTable) })

	_, err := database.Exec("CREATE TABLE " + quotedTable + ` (
		id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		name TEXT NOT NULL,
		enabled BOOLEAN,
		note TEXT
	)`)
	if err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := database.Exec(
			"INSERT INTO "+quotedTable+" (name, enabled, note) VALUES ($1, $2, $3)",
			fmt.Sprintf("record-%d", i), i%2 == 0, nil,
		); err != nil {
			t.Fatalf("insert fixture row: %v", err)
		}
	}

	tables, err := GetTables(database)
	if err != nil {
		t.Fatalf("get tables: %v", err)
	}
	if !slices.Contains(tables, tableName) {
		t.Fatalf("fixture table missing from discovery result: %v", tables)
	}

	columns, rows, err := GetTableDataPage(database, tableName, 2, 2)
	if err != nil {
		t.Fatalf("get paginated table data: %v", err)
	}
	if len(columns) != 4 || len(rows) != 2 {
		t.Fatalf("unexpected page dimensions: %d columns, %d rows", len(columns), len(rows))
	}
	if columns[0].Title != "id" || columns[1].Title != "name" {
		t.Fatalf("unexpected columns: %#v", columns)
	}

	firstPage, err := BrowseTable(database, tableName, TableBrowseRequest{Limit: 2})
	if err != nil {
		t.Fatalf("browse first keyset page: %v", err)
	}
	if !firstPage.KeysetEnabled || !firstPage.HasNext || firstPage.NextCursor != "2" || firstPage.Rows[0][0] != "1" {
		t.Fatalf("unexpected first keyset page: %#v", firstPage)
	}
	if len(firstPage.Structure) != 4 || firstPage.Structure[0].Name != "id" {
		t.Fatalf("browse result did not carry its discovered structure: %#v", firstPage.Structure)
	}
	secondPage, err := BrowseTable(database, tableName, TableBrowseRequest{Limit: 2, Cursor: firstPage.NextCursor})
	if err != nil {
		t.Fatalf("browse second keyset page: %v", err)
	}
	if len(secondPage.Rows) != 2 || secondPage.Rows[0][0] != "3" || secondPage.Rows[1][0] != "4" {
		t.Fatalf("unexpected second keyset page: %#v", secondPage.Rows)
	}
	filtered, err := BrowseTable(database, tableName, TableBrowseRequest{
		Limit: 10, FilterColumn: "enabled", FilterValue: "true",
	})
	if err != nil {
		t.Fatalf("browse filtered rows: %v", err)
	}
	if len(filtered.Rows) != 2 || filtered.Rows[0][0] != "2" || filtered.Rows[1][0] != "4" {
		t.Fatalf("unexpected filtered rows: %#v", filtered.Rows)
	}
	sorted, err := BrowseTable(database, tableName, TableBrowseRequest{
		Limit: 2, SortColumn: "name", Descending: true,
	})
	if err != nil {
		t.Fatalf("browse sorted rows: %v", err)
	}
	if sorted.KeysetEnabled || sorted.Rows[0][1] != "record-5" || sorted.Rows[1][1] != "record-4" {
		t.Fatalf("unexpected custom sort: %#v", sorted)
	}
	if _, err := BrowseTable(database, tableName, TableBrowseRequest{FilterColumn: "id; DROP TABLE unsafe", FilterValue: "1"}); err == nil {
		t.Fatal("unsafe filter column was accepted")
	}

	allColumns, err := GetAllColumns(database)
	if err != nil {
		t.Fatalf("get all columns: %v", err)
	}
	for _, expected := range []string{"id", "name", "enabled", "note"} {
		if !slices.Contains(allColumns, expected) {
			t.Errorf("column %q missing from schema discovery", expected)
		}
	}

	structure, err := GetTableStructure(database, tableName)
	if err != nil {
		t.Fatalf("get table structure: %v", err)
	}
	if len(structure) != 4 || structure[0].Name != "id" || !structure[0].PrimaryKey || !structure[3].Nullable {
		t.Fatalf("unexpected table structure: %#v", structure)
	}

	cache, err := NewSchemaCache(database)
	if err != nil {
		t.Fatalf("build schema cache: %v", err)
	}
	if !slices.Contains(cache.Tables, tableName) {
		t.Fatalf("fixture table missing from schema cache: %v", cache.Tables)
	}
	if got := cache.Columns[tableName]; !slices.Equal(got, []string{"id", "name", "enabled", "note"}) {
		t.Fatalf("unexpected cached columns: %v", got)
	}
}

func TestIntegrationExecuteQuery(t *testing.T) {
	database, _ := openIntegrationDatabase(t)
	result := ExecuteQuery(database, "SELECT 42 AS answer, 'maxim' AS project")
	if !result.Success {
		t.Fatalf("query failed: %s", result.Error)
	}
	if result.RowCount != 1 || !strings.Contains(result.Data, "answer") || !strings.Contains(result.Data, "maxim") {
		t.Fatalf("unexpected query result: %#v", result)
	}

	invalid := ExecuteQuery(database, "SELECT * FROM table_that_does_not_exist")
	if invalid.Success || invalid.Error == "" {
		t.Fatalf("invalid query did not return an error: %#v", invalid)
	}
}

func TestIntegrationQueryHonorsCancellation(t *testing.T) {
	database, _ := openIntegrationDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := ExecuteQueryContext(ctx, database, "SELECT pg_sleep(10)")
	if result.Success || !strings.Contains(strings.ToLower(result.Error), "cancel") {
		t.Fatalf("cancelled query returned %#v", result)
	}
}

func TestIntegrationMutationsAndPostgresDataTypes(t *testing.T) {
	database, _ := openIntegrationDatabase(t)
	tableName := uniqueDatabaseObject("types")
	quotedTable := pq.QuoteIdentifier(tableName)
	t.Cleanup(func() { _, _ = database.Exec("DROP TABLE IF EXISTS " + quotedTable) })

	_, err := database.Exec("CREATE TABLE " + quotedTable + ` (
		id UUID PRIMARY KEY,
		metadata JSONB NOT NULL,
		tags TEXT[] NOT NULL,
		created_at TIMESTAMPTZ NOT NULL,
		payload BYTEA
	)`)
	if err != nil {
		t.Fatalf("create data-type fixture: %v", err)
	}

	insert := ExecuteQuery(database, "INSERT INTO "+quotedTable+` (id, metadata, tags, created_at, payload)
		VALUES ('123e4567-e89b-12d3-a456-426614174000', '{"source":"maxim"}', ARRAY['cli','test'],
		'2026-09-09T10:00:00Z', decode('4d6178696d', 'hex')) RETURNING id, metadata`)
	if !insert.Success || insert.RowCount != 1 || !strings.Contains(insert.Data, "source") {
		t.Fatalf("insert with RETURNING failed: %#v", insert)
	}

	update := ExecuteQuery(database, "UPDATE "+quotedTable+` SET metadata = '{"source":"integration"}' RETURNING id`)
	if !update.Success || update.RowCount != 1 {
		t.Fatalf("update with RETURNING failed: %#v", update)
	}

	columns, rows, err := GetTableData(database, tableName)
	if err != nil {
		t.Fatalf("read PostgreSQL data types: %v", err)
	}
	if len(columns) != 5 || len(rows) != 1 {
		t.Fatalf("unexpected typed data result: %d columns, %d rows", len(columns), len(rows))
	}

	remove := ExecuteQuery(database, "DELETE FROM "+quotedTable+" RETURNING id")
	if !remove.Success || remove.RowCount != 1 {
		t.Fatalf("delete with RETURNING failed: %#v", remove)
	}
}

func TestIntegrationCreateAndDeleteDatabase(t *testing.T) {
	admin, config := openIntegrationDatabase(t)
	databaseName := uniqueDatabaseObject("database")
	roleName := uniqueDatabaseObject("role")
	rolePassword := "MaximIntegration_42"

	t.Cleanup(func() {
		_, _ = admin.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", databaseName)
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + pq.QuoteIdentifier(databaseName))
		_, _ = admin.Exec("DROP ROLE IF EXISTS " + pq.QuoteIdentifier(roleName))
	})

	if err := CreateDBAndUser(
		admin, "psql", databaseName, roleName, rolePassword,
		config.user, config.password, config.host, config.port,
	); err != nil {
		t.Fatalf("create database and role: %v", err)
	}

	created, err := ConnectPostgres(ConnectionOptions{
		Host: config.host, Port: config.port, User: roleName,
		Password: rolePassword, Database: databaseName, SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("connect as newly created role: %v", err)
	}
	if _, err := created.Exec("CREATE TABLE ownership_check (id INTEGER PRIMARY KEY)"); err != nil {
		created.Close()
		t.Fatalf("new role cannot create tables: %v", err)
	}
	created.Close()

	if err := DeleteDatabase(admin, "psql", databaseName); err != nil {
		t.Fatalf("delete database: %v", err)
	}
	var databaseExists, roleExists bool
	if err := admin.QueryRow("SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", databaseName).Scan(&databaseExists); err != nil {
		t.Fatal(err)
	}
	if err := admin.QueryRow("SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)", roleName).Scan(&roleExists); err != nil {
		t.Fatal(err)
	}
	if databaseExists || roleExists {
		t.Fatalf("cleanup incomplete: database exists=%t, role exists=%t", databaseExists, roleExists)
	}
}
