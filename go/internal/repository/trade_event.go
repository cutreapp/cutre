package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// TradeEventRepository はtrade_events (交換の出来事) を読み書きする。
type TradeEventRepository struct {
	q *query.Queries
}

// NewTradeEventRepository は TradeEventRepository を生成する。
func NewTradeEventRepository(db *sql.DB) *TradeEventRepository {
	return &TradeEventRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい TradeEventRepository を返す。
func (r *TradeEventRepository) WithTx(tx *sql.Tx) *TradeEventRepository {
	return &TradeEventRepository{q: r.q.WithTx(tx)}
}

// Create は、ユーザー actorUserID が交換 tradeID で起こした出来事 kind を記録する。
// reason は選んだ理由の選択肢のキーで、理由を選ばない出来事ではnilを渡す。
func (r *TradeEventRepository) Create(ctx context.Context, tradeID model.TradeID, actorUserID model.UserID, kind model.TradeEventKind, reason *string) (*model.TradeEvent, error) {
	row, err := r.q.CreateTradeEvent(ctx, query.CreateTradeEventParams{
		TradeID:     uuid.UUID(tradeID),
		ActorUserID: uuid.UUID(actorUserID),
		Kind:        query.TradeEventKind(kind),
		Reason:      reason,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// ListByTradeID は交換 tradeID の出来事を、起きた順に返す。
func (r *TradeEventRepository) ListByTradeID(ctx context.Context, tradeID model.TradeID) ([]*model.TradeEvent, error) {
	rows, err := r.q.ListTradeEventsByTradeID(ctx, uuid.UUID(tradeID))
	if err != nil {
		return nil, err
	}

	events := make([]*model.TradeEvent, len(rows))
	for i, row := range rows {
		events[i] = r.toModel(row)
	}

	return events, nil
}

// toModel はクエリの行を model.TradeEvent に変換する。
func (r *TradeEventRepository) toModel(row query.TradeEvent) *model.TradeEvent {
	return &model.TradeEvent{
		ID:          model.TradeEventID(row.ID),
		TradeID:     model.TradeID(row.TradeID),
		ActorUserID: model.UserID(row.ActorUserID),
		Kind:        model.TradeEventKind(row.Kind),
		Reason:      row.Reason,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
