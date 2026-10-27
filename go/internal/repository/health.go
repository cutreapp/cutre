package repository

import (
	"context"
	"database/sql"

	"github.com/cutreapp/cutre/go/internal/query"
)

// HealthRepository はヘルスチェックのためにデータベースへ問い合わせる。
type HealthRepository struct {
	q *query.Queries
}

// NewHealthRepository は HealthRepository を生成する。
func NewHealthRepository(db *sql.DB) *HealthRepository {
	return &HealthRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい HealthRepository を返す。
func (r *HealthRepository) WithTx(tx *sql.Tx) *HealthRepository {
	return &HealthRepository{q: r.q.WithTx(tx)}
}

// Ping はデータベースへクエリを往復させ、応答できるかを確かめる。
func (r *HealthRepository) Ping(ctx context.Context) error {
	_, err := r.q.Ping(ctx)
	return err
}
