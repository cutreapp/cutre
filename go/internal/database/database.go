// Package database はPostgreSQLへの接続を管理する。
package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Connect はDSN (DATABASE_URL形式の接続文字列) でPostgreSQLに接続し、疎通を確認する。
// sql.Open は接続を張らずに戻るため、Pingまで行って起動時に接続できないことを検出する。
func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("データベースへの接続に失敗しました: %w", err)
	}

	if err := db.Ping(); err != nil {
		// 呼び出し元は失敗時に *sql.DB を受け取らないため、ここで閉じないとプールが残る。
		_ = db.Close()
		return nil, fmt.Errorf("データベースへのPingに失敗しました: %w", err)
	}

	return db, nil
}

// サーバー (cutre serve) のdatabase/sqlの接続プールの設定。値はAnnictにそろえる。
//
// PostgreSQLの max_connections は、デプロイの切り替え中に新旧2つのプロセスが並ぶ前提で、
// (MaxOpenConns + ワーカーの接続プールの上限 (internal/worker の poolMaxConns)) × 2 に、
// 運用の接続 (Postico・cutreコマンド・postgres_exporter・復旧作業) の余裕を足した数以上にする。
// いまの値では (25 + 10) × 2 = 70 で、既定の100なら30の余裕が残る。
const (
	MaxOpenConns    = 25
	MaxIdleConns    = 5
	ConnMaxLifetime = 5 * time.Minute
	ConnMaxIdleTime = 2 * time.Minute
)

// SetServerPoolLimits はサーバーが使う接続プールに上限を設ける。
// 上限が無いと、負荷が上がったときに開く接続が増え続け、PostgreSQLの max_connections を使い切って
// 同居するワーカーや運用の接続まで締め出すため。
// 単発で動くcutreコマンドとテストは、使う接続がわずかなため上限を設けない。
func SetServerPoolLimits(db *sql.DB) {
	db.SetMaxOpenConns(MaxOpenConns)
	db.SetMaxIdleConns(MaxIdleConns)
	db.SetConnMaxLifetime(ConnMaxLifetime)
	db.SetConnMaxIdleTime(ConnMaxIdleTime)
}
