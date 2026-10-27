package metrics

import (
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// RegisterDB はdatabase/sqlの接続プールの状態 (go_sql_*) を登録する。
// nameはラベル db_name の値で、プールを見分けるために使う。
//
// 接続の待ちの件数と時間 (go_sql_wait_count_total・go_sql_wait_duration_seconds_total) も含む。
// 接続の上限が詰まっているかは、これで判断する。
func (r *Registry) RegisterDB(db *sql.DB, name string) {
	r.reg.MustRegister(collectors.NewDBStatsCollector(db, name))
}

// RegisterPgxPool はpgxの接続プールの状態 (pgxpool_*) を登録する。
// nameはラベル pool_name の値で、プールを見分けるために使う。
//
// プールそのものではなく状態を返す関数を受け取るのは、呼び出し元がプールを外へ渡さずに済むようにするため。
func (r *Registry) RegisterPgxPool(stat func() *pgxpool.Stat, name string) {
	r.reg.MustRegister(newPgxPoolCollector(stat, name))
}

// pgxPoolCollector は収集のたびにpgxの接続プールの状態を読み取る。
type pgxPoolCollector struct {
	stat func() *pgxpool.Stat

	acquireCount            *prometheus.Desc
	acquireDuration         *prometheus.Desc
	emptyAcquireCount       *prometheus.Desc
	emptyAcquireWaitTime    *prometheus.Desc
	canceledAcquireCount    *prometheus.Desc
	acquiredConns           *prometheus.Desc
	idleConns               *prometheus.Desc
	constructingConns       *prometheus.Desc
	totalConns              *prometheus.Desc
	maxConns                *prometheus.Desc
	newConnsCount           *prometheus.Desc
	maxLifetimeDestroyCount *prometheus.Desc
	maxIdleDestroyCount     *prometheus.Desc
}

func newPgxPoolCollector(stat func() *pgxpool.Stat, name string) *pgxPoolCollector {
	desc := func(metric, help string) *prometheus.Desc {
		return prometheus.NewDesc("pgxpool_"+metric, help, nil, prometheus.Labels{"pool_name": name})
	}

	return &pgxPoolCollector{
		stat: stat,

		acquireCount:            desc("acquire_count_total", "プールから接続を取り出した回数。"),
		acquireDuration:         desc("acquire_duration_seconds_total", "接続の取り出しにかかった時間の合計 (秒)。"),
		emptyAcquireCount:       desc("empty_acquire_count_total", "空きの接続が無く、待ってから取り出した回数。"),
		emptyAcquireWaitTime:    desc("empty_acquire_wait_seconds_total", "空きの接続が無いときに待った時間の合計 (秒)。"),
		canceledAcquireCount:    desc("canceled_acquire_count_total", "取り出しを待つ間にcontextが取り消された回数。"),
		acquiredConns:           desc("acquired_connections", "取り出されて使用中の接続の数。"),
		idleConns:               desc("idle_connections", "空いている接続の数。"),
		constructingConns:       desc("constructing_connections", "確立している途中の接続の数。"),
		totalConns:              desc("total_connections", "プールが持つ接続の数 (使用中・空き・確立中の合計)。"),
		maxConns:                desc("max_connections", "プールが持てる接続の上限。"),
		newConnsCount:           desc("new_connections_total", "新しく確立した接続の数。"),
		maxLifetimeDestroyCount: desc("max_lifetime_destroy_count_total", "最大の寿命を過ぎて閉じた接続の数。"),
		maxIdleDestroyCount:     desc("max_idle_destroy_count_total", "空いている時間が上限を過ぎて閉じた接続の数。"),
	}
}

// Describe はprometheus.Collectorを実装する。
func (c *pgxPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect はprometheus.Collectorを実装する。
func (c *pgxPoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.stat()

	counter := func(desc *prometheus.Desc, value float64) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.CounterValue, value)
	}
	gauge := func(desc *prometheus.Desc, value float64) {
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
	}

	counter(c.acquireCount, float64(s.AcquireCount()))
	counter(c.acquireDuration, s.AcquireDuration().Seconds())
	counter(c.emptyAcquireCount, float64(s.EmptyAcquireCount()))
	counter(c.emptyAcquireWaitTime, s.EmptyAcquireWaitTime().Seconds())
	counter(c.canceledAcquireCount, float64(s.CanceledAcquireCount()))
	gauge(c.acquiredConns, float64(s.AcquiredConns()))
	gauge(c.idleConns, float64(s.IdleConns()))
	gauge(c.constructingConns, float64(s.ConstructingConns()))
	gauge(c.totalConns, float64(s.TotalConns()))
	gauge(c.maxConns, float64(s.MaxConns()))
	counter(c.newConnsCount, float64(s.NewConnsCount()))
	counter(c.maxLifetimeDestroyCount, float64(s.MaxLifetimeDestroyCount()))
	counter(c.maxIdleDestroyCount, float64(s.MaxIdleDestroyCount()))
}
