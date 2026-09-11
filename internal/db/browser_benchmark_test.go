//go:build integration

package db

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func benchmarkRowCount(b *testing.B) int {
	b.Helper()
	value := os.Getenv("MAXIM_BENCH_ROWS")
	if value == "" {
		b.Skip("set MAXIM_BENCH_ROWS=100000 or 1000000 to run large database benchmarks")
	}
	rows, err := strconv.Atoi(value)
	if err != nil || (rows != 100000 && rows != 1000000) {
		b.Fatalf("MAXIM_BENCH_ROWS must be 100000 or 1000000, got %q", value)
	}
	return rows
}

func openBenchmarkDatabase(b *testing.B) *sql.DB {
	b.Helper()
	config := integrationDatabaseConfig{
		host: os.Getenv("MAXIM_TEST_DB_HOST"), port: os.Getenv("MAXIM_TEST_DB_PORT"),
		user: os.Getenv("MAXIM_TEST_DB_USER"), password: os.Getenv("MAXIM_TEST_DB_PASSWORD"),
		database: os.Getenv("MAXIM_TEST_DB_NAME"),
	}
	if config.host == "" || config.port == "" || config.user == "" || config.database == "" {
		b.Fatal("benchmark database is not configured; set MAXIM_TEST_DB_* variables")
	}
	database, err := ConnectPostgres(ConnectionOptions{
		Host: config.host, Port: config.port, User: config.user, Password: config.password,
		Database: config.database, SSLMode: "disable",
	})
	if err != nil {
		b.Fatalf("connect to benchmark database: %v", err)
	}
	b.Cleanup(func() { database.Close() })
	return database
}

func createLargeBenchmarkFixture(b *testing.B, database *sql.DB, rows int) Relation {
	b.Helper()
	relation := Relation{Schema: "public", Name: uniqueDatabaseObject("browse_bench")}
	b.Cleanup(func() { _, _ = database.Exec("DROP TABLE IF EXISTS " + relation.QualifiedName()) })
	_, err := database.Exec("CREATE TABLE " + relation.QualifiedName() + ` (
		tenant_id INTEGER NOT NULL,
		id BIGINT NOT NULL,
		rank INTEGER NOT NULL,
		note TEXT,
		payload JSONB NOT NULL,
		unicode_text TEXT NOT NULL,
		long_text TEXT NOT NULL,
		PRIMARY KEY (tenant_id, id)
	)`)
	if err != nil {
		b.Fatalf("create benchmark fixture: %v", err)
	}
	_, err = database.Exec("INSERT INTO "+relation.QualifiedName()+` (tenant_id, id, rank, note, payload, unicode_text, long_text)
		SELECT (value % 100)::integer,
		       value,
		       (value % 1000)::integer,
		       CASE WHEN value % 7 = 0 THEN NULL ELSE 'note-' || (value % 50)::text END,
		       jsonb_build_object('id', value, 'tags', jsonb_build_array('fixture', value % 10)),
		       'नमस्ते-世界-🚀-' || value::text,
		       CASE WHEN value % 1000 = 0 THEN repeat('large-value-', 10000) ELSE 'small-' || value::text END
		FROM generate_series(1, $1) AS value`, rows)
	if err != nil {
		b.Fatalf("populate %d-row benchmark fixture: %v", rows, err)
	}
	if _, err := database.Exec("CREATE INDEX " + relation.Name + "_rank_pk_idx ON " + relation.QualifiedName() + " (rank, tenant_id, id)"); err != nil {
		b.Fatalf("create browse index: %v", err)
	}
	if _, err := database.Exec("ANALYZE " + relation.QualifiedName()); err != nil {
		b.Fatalf("analyze benchmark fixture: %v", err)
	}
	return relation
}

func logBenchmarkPlan(b *testing.B, database *sql.DB, relation Relation) {
	b.Helper()
	rows, err := database.Query("EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT * FROM " + relation.QualifiedName() + " WHERE (rank, tenant_id, id) > (900, 0, 0) ORDER BY rank, tenant_id, id LIMIT 101")
	if err != nil {
		b.Fatalf("explain benchmark query: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			b.Fatalf("scan query plan: %v", err)
		}
		plan = append(plan, line)
	}
	b.Logf("environment: os=%s arch=%s cpus=%d go=%s rows=%s\nquery plan:\n%s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), os.Getenv("MAXIM_BENCH_ROWS"), strings.Join(plan, "\n"))
}

func BenchmarkBrowseLargeFixture(b *testing.B) {
	rows := benchmarkRowCount(b)
	database := openBenchmarkDatabase(b)
	relation := createLargeBenchmarkFixture(b, database, rows)
	logBenchmarkPlan(b, database, relation)
	b.ReportMetric(float64(rows), "fixture_rows")

	b.Run("indexed_keyset_first_page", func(b *testing.B) {
		request := TableBrowseRequest{Limit: 100, SortColumn: "rank", ConnectionID: 1}
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			if _, err := BrowseRelation(database, relation, request); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("indexed_keyset_deep_page", func(b *testing.B) {
		request := TableBrowseRequest{Limit: 100, SortColumn: "rank", ConnectionID: 2, Cursor: &TableCursor{
			Values:       []CellValue{{Raw: int64(900), Text: "900"}, {Raw: int64(0), Text: "0"}, {Raw: int64(0), Text: "0"}},
			OrderColumns: []string{"rank", "tenant_id", "id"}, ConnectionID: 2, Relation: relation, SortColumn: "rank",
		}}
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			if _, err := BrowseRelation(database, relation, request); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("nullable_offset_deep_page", func(b *testing.B) {
		request := TableBrowseRequest{Limit: 100, Offset: rows * 9 / 10, SortColumn: "note", ConnectionID: 3}
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			if _, err := BrowseRelation(database, relation, request); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("keyset_during_writes", func(b *testing.B) {
		ctx, cancel := context.WithCancel(context.Background())
		var nextID atomic.Int64
		nextID.Store(int64(rows + 1))
		var writer sync.WaitGroup
		writer.Add(1)
		go func() {
			defer writer.Done()
			for ctx.Err() == nil {
				id := nextID.Add(1)
				_, _ = database.ExecContext(ctx, "INSERT INTO "+relation.QualifiedName()+" (tenant_id, id, rank, payload, unicode_text, long_text) VALUES ($1, $2, $3, '{}'::jsonb, 'concurrent-世界', 'small') ON CONFLICT DO NOTHING", id%100, id, id%1000)
			}
		}()
		request := TableBrowseRequest{Limit: 100, SortColumn: "rank", ConnectionID: 4, Timeout: 30 * time.Second}
		b.ReportAllocs()
		b.ResetTimer()
		for index := 0; index < b.N; index++ {
			if _, err := BrowseRelation(database, relation, request); err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
		cancel()
		writer.Wait()
	})
}
