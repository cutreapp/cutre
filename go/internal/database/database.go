// Package database はPostgreSQLへの接続を管理する。
package database

import (
	"database/sql"
	"fmt"

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
