package metrics_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cutreapp/cutre/go/internal/metrics"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestRegistry_RegisterDB は、database/sqlの接続プールの状態を、待ちの件数と時間を含めて返すことを検証する。
func TestRegistry_RegisterDB(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	reg.RegisterDB(testutil.GetTestDB(), "main")

	_, body := scrape(t, reg, http.MethodGet, metrics.Path)
	for _, want := range []string{
		`go_sql_open_connections{db_name="main"} `,
		`go_sql_in_use_connections{db_name="main"} `,
		`go_sql_wait_count_total{db_name="main"} `,
		`go_sql_wait_duration_seconds_total{db_name="main"} `,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("メトリクスに %s が含まれない", want)
		}
	}
}

// TestRegistry_RegisterPgxPool は、pgxの接続プールの状態を収集のたびに読み取って返すことを検証する。
func TestRegistry_RegisterPgxPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("接続文字列の解析のエラー = %v", err)
	}
	poolConfig.MaxConns = 3
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("接続プールの作成のエラー = %v", err)
	}
	t.Cleanup(pool.Close)

	reg := metrics.NewRegistry()
	reg.RegisterPgxPool(pool.Stat, "river")

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("接続の取り出しのエラー = %v", err)
	}
	_, body := scrape(t, reg, http.MethodGet, metrics.Path)
	for _, want := range []string{
		`pgxpool_acquire_count_total{pool_name="river"} 1`,
		`pgxpool_acquired_connections{pool_name="river"} 1`,
		`pgxpool_max_connections{pool_name="river"} 3`,
		`pgxpool_empty_acquire_wait_seconds_total{pool_name="river"} `,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("取り出し中のメトリクスに %s が含まれない\n%s", want, body)
		}
	}

	conn.Release()
	_, body = scrape(t, reg, http.MethodGet, metrics.Path)
	for _, want := range []string{
		`pgxpool_acquired_connections{pool_name="river"} 0`,
		`pgxpool_idle_connections{pool_name="river"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("返却後のメトリクスに %s が含まれない\n%s", want, body)
		}
	}
}
