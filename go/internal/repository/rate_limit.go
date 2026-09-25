package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// RateLimitRepository はrate_limitsを読み書きする。
type RateLimitRepository struct {
	q *query.Queries
}

// NewRateLimitRepository は RateLimitRepository を生成する。
func NewRateLimitRepository(db *sql.DB) *RateLimitRepository {
	return &RateLimitRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい RateLimitRepository を返す。
func (r *RateLimitRepository) WithTx(tx *sql.Tx) *RateLimitRepository {
	return &RateLimitRepository{q: r.q.WithTx(tx)}
}

// Increment は指定したキーと時間枠のカウンターを1つ増やし、増やしたあとのカウンターを返す。
// その枠のカウンターがまだ無ければ1で作る。
func (r *RateLimitRepository) Increment(ctx context.Context, key string, windowStart time.Time) (*model.RateLimit, error) {
	row, err := r.q.IncrementRateLimit(ctx, query.IncrementRateLimitParams{
		Key:         key,
		WindowStart: windowStart,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// DeleteOlderThan は指定した時刻より前に始まった時間枠のカウンターを削除する。
// 過ぎた枠のカウンターは判定に使われないため、残しておく必要がない。
func (r *RateLimitRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) error {
	return r.q.DeleteRateLimitsOlderThan(ctx, cutoff)
}

// toModel はクエリの行を model.RateLimit に変換する。
func (r *RateLimitRepository) toModel(row query.RateLimit) *model.RateLimit {
	return &model.RateLimit{
		ID:          model.RateLimitID(row.ID),
		Key:         row.Key,
		WindowStart: row.WindowStart,
		Count:       row.Count,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
