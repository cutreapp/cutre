package worker

import "github.com/jackc/pgx/v5/pgxpool"

// PoolConfig は、クライアントが開いた接続プールの設定を返す。
// 接続数の上限を明示していることをテストで確かめるために使う。
func (c *Client) PoolConfig() *pgxpool.Config {
	return c.pool.Config()
}
