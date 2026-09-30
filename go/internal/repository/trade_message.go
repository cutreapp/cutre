package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// TradeMessageRepository はtrade_messages (交換のメッセージ) を読み書きする。
type TradeMessageRepository struct {
	q *query.Queries
}

// NewTradeMessageRepository は TradeMessageRepository を生成する。
func NewTradeMessageRepository(db *sql.DB) *TradeMessageRepository {
	return &TradeMessageRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい TradeMessageRepository を返す。
func (r *TradeMessageRepository) WithTx(tx *sql.Tx) *TradeMessageRepository {
	return &TradeMessageRepository{q: r.q.WithTx(tx)}
}

// Create は、ユーザー senderUserID が交換 tradeID で本文 body のメッセージを送ったことを記録する。
func (r *TradeMessageRepository) Create(ctx context.Context, tradeID model.TradeID, senderUserID model.UserID, body string) (*model.TradeMessage, error) {
	row, err := r.q.CreateTradeMessage(ctx, query.CreateTradeMessageParams{
		TradeID:      uuid.UUID(tradeID),
		SenderUserID: uuid.UUID(senderUserID),
		Body:         body,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// CreateInProgress は、ユーザー senderUserID が進行中 (返事待ち・マッチ成立) の交換 tradeID で本文 body のメッセージを送ったことを記録する。
// 交換が無いか終わっていたときは記録せず、(nil, nil) を返す。
// 交換の行を共有ロックで読んで段階を確かめるため、交換を終える操作と同時でも、終わった交換には記録しない。
func (r *TradeMessageRepository) CreateInProgress(ctx context.Context, tradeID model.TradeID, senderUserID model.UserID, body string) (*model.TradeMessage, error) {
	row, err := r.q.CreateTradeMessageInProgress(ctx, query.CreateTradeMessageInProgressParams{
		SenderUserID: uuid.UUID(senderUserID),
		Body:         body,
		TradeID:      uuid.UUID(tradeID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListByTradeID は交換 tradeID のメッセージを、送った順に返す。取り消したメッセージも返す。
func (r *TradeMessageRepository) ListByTradeID(ctx context.Context, tradeID model.TradeID) ([]*model.TradeMessage, error) {
	rows, err := r.q.ListTradeMessagesByTradeID(ctx, uuid.UUID(tradeID))
	if err != nil {
		return nil, err
	}

	messages := make([]*model.TradeMessage, len(rows))
	for i, row := range rows {
		messages[i] = r.toModel(row)
	}

	return messages, nil
}

// FindByID はメッセージ id を返す。無ければ (nil, nil) を返す。
func (r *TradeMessageRepository) FindByID(ctx context.Context, id model.TradeMessageID) (*model.TradeMessage, error) {
	row, err := r.q.GetTradeMessageByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListLatestByTradeIDs は、交換 tradeIDs ごとの最新のメッセージを返す。取り消したメッセージも返す。
// メッセージの無い交換はmapに入らない。tradeIDs が空ならクエリを発行せずにnilを返す。
func (r *TradeMessageRepository) ListLatestByTradeIDs(ctx context.Context, tradeIDs []model.TradeID) (map[model.TradeID]*model.TradeMessage, error) {
	if len(tradeIDs) == 0 {
		return nil, nil
	}

	rows, err := r.q.ListLatestTradeMessagesByTradeIDs(ctx, tradeUUIDs(tradeIDs))
	if err != nil {
		return nil, err
	}

	messages := make(map[model.TradeID]*model.TradeMessage, len(rows))
	for _, row := range rows {
		message := r.toModel(row)
		messages[message.TradeID] = message
	}

	return messages, nil
}

// CountUnreadByTradeIDs は、ユーザー userID の交換 tradeIDs ごとの未読のメッセージの数を返す。
// 相手が送ったメッセージのうち、最後に読んだ時刻より後のものを数え、取り消したものは数えない。
// 未読の無い交換はmapに入らない。tradeIDs が空ならクエリを発行せずにnilを返す。
func (r *TradeMessageRepository) CountUnreadByTradeIDs(ctx context.Context, userID model.UserID, tradeIDs []model.TradeID) (map[model.TradeID]int64, error) {
	if len(tradeIDs) == 0 {
		return nil, nil
	}

	rows, err := r.q.CountUnreadTradeMessagesByTradeIDs(ctx, query.CountUnreadTradeMessagesByTradeIDsParams{
		UserID:   uuid.UUID(userID),
		TradeIds: tradeUUIDs(tradeIDs),
	})
	if err != nil {
		return nil, err
	}

	counts := make(map[model.TradeID]int64, len(rows))
	for _, row := range rows {
		counts[model.TradeID(row.TradeID)] = row.UnreadCount
	}

	return counts, nil
}

// CountUnreadByUserID は、ユーザー userID の交換すべての未読のメッセージの数を返す。数え方は CountUnreadByTradeIDs と同じ。
func (r *TradeMessageRepository) CountUnreadByUserID(ctx context.Context, userID model.UserID) (int64, error) {
	return r.q.CountUnreadTradeMessagesByUserID(ctx, uuid.UUID(userID))
}

// Retract は、ユーザー senderUserID が送ったメッセージ id を取り消す。本文は消さずに残す。
// 送った人が違うか、すでに取り消していたときは取り消さず、falseを返す。
func (r *TradeMessageRepository) Retract(ctx context.Context, id model.TradeMessageID, senderUserID model.UserID) (bool, error) {
	affected, err := r.q.RetractTradeMessage(ctx, query.RetractTradeMessageParams{
		ID:           uuid.UUID(id),
		SenderUserID: uuid.UUID(senderUserID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// tradeUUIDs は交換のIDをクエリに渡すuuidの配列にする。
func tradeUUIDs(ids []model.TradeID) []uuid.UUID {
	uuids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uuids[i] = uuid.UUID(id)
	}

	return uuids
}

// toModel はクエリの行を model.TradeMessage に変換する。
func (r *TradeMessageRepository) toModel(row query.TradeMessage) *model.TradeMessage {
	return &model.TradeMessage{
		ID:           model.TradeMessageID(row.ID),
		TradeID:      model.TradeID(row.TradeID),
		SenderUserID: model.UserID(row.SenderUserID),
		Body:         row.Body,
		RetractedAt:  row.RetractedAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
