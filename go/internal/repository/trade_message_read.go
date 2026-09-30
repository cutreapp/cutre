package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// TradeMessageReadRepository はtrade_message_reads (交換のメッセージをどこまで読んだか) を読み書きする。
type TradeMessageReadRepository struct {
	q *query.Queries
}

// NewTradeMessageReadRepository は TradeMessageReadRepository を生成する。
func NewTradeMessageReadRepository(db *sql.DB) *TradeMessageReadRepository {
	return &TradeMessageReadRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい TradeMessageReadRepository を返す。
func (r *TradeMessageReadRepository) WithTx(tx *sql.Tx) *TradeMessageReadRepository {
	return &TradeMessageReadRepository{q: r.q.WithTx(tx)}
}

// FindLastReadPosition は、ユーザー userID が交換 tradeID のメッセージをどこまで読んだかを返す。
// まだ読んだことが無ければ (nil, nil) を返す。
func (r *TradeMessageReadRepository) FindLastReadPosition(ctx context.Context, tradeID model.TradeID, userID model.UserID) (*model.TradeMessageReadPosition, error) {
	row, err := r.q.GetTradeMessageReadByTradeIDAndUserID(ctx, query.GetTradeMessageReadByTradeIDAndUserIDParams{
		TradeID: uuid.UUID(tradeID),
		UserID:  uuid.UUID(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &model.TradeMessageReadPosition{CreatedAt: row.LastReadAt, MessageID: model.TradeMessageID(row.LastReadMessageID)}, nil
}

// Save は、ユーザー userID が交換 tradeID のメッセージを position まで読んだことを記録する。
// すでにそれより後まで読んでいたときは、読んだ位置を戻さない。
func (r *TradeMessageReadRepository) Save(ctx context.Context, tradeID model.TradeID, userID model.UserID, position model.TradeMessageReadPosition) error {
	return r.q.UpsertTradeMessageRead(ctx, query.UpsertTradeMessageReadParams{
		TradeID:           uuid.UUID(tradeID),
		UserID:            uuid.UUID(userID),
		LastReadAt:        position.CreatedAt,
		LastReadMessageID: uuid.UUID(position.MessageID),
	})
}
