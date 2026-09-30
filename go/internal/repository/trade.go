package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// TradeRepository はtrades (交換) を読み書きする。
type TradeRepository struct {
	q *query.Queries
}

// NewTradeRepository は TradeRepository を生成する。
func NewTradeRepository(db *sql.DB) *TradeRepository {
	return &TradeRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい TradeRepository を返す。
func (r *TradeRepository) WithTx(tx *sql.Tx) *TradeRepository {
	return &TradeRepository{q: r.q.WithTx(tx)}
}

// Create は、ユーザー proposerUserID がユーザー receiverUserID に申し込んだ、返事待ちの交換を作る。
func (r *TradeRepository) Create(ctx context.Context, proposerUserID, receiverUserID model.UserID) (*model.Trade, error) {
	row, err := r.q.CreateTrade(ctx, query.CreateTradeParams{
		ProposerUserID: uuid.UUID(proposerUserID),
		ReceiverUserID: uuid.UUID(receiverUserID),
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// ExistsInProgressByUserID は、ユーザー userID が申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換があるかを返す。
func (r *TradeRepository) ExistsInProgressByUserID(ctx context.Context, userID model.UserID) (bool, error) {
	return r.q.ExistsInProgressTradeByUserID(ctx, uuid.UUID(userID))
}

// CountInProgressByUserID は、ユーザー userID が申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換の数を返す。
func (r *TradeRepository) CountInProgressByUserID(ctx context.Context, userID model.UserID) (int64, error) {
	return r.q.CountInProgressTradesByUserID(ctx, uuid.UUID(userID))
}

// FindByID は指定したIDの交換を返す。存在しない場合は (nil, nil) を返す。
// 交換の2人以外に見せないかは呼び出し側が決める。
func (r *TradeRepository) FindByID(ctx context.Context, id model.TradeID) (*model.Trade, error) {
	row, err := r.q.GetTradeByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// ListInProgressByUserID は、ユーザー userID が申し込んだか申し込まれた、進行中 (返事待ち・マッチ成立) の交換を、新しく申し込まれた順に返す。
func (r *TradeRepository) ListInProgressByUserID(ctx context.Context, userID model.UserID) ([]*model.Trade, error) {
	rows, err := r.q.ListInProgressTradesByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	trades := make([]*model.Trade, len(rows))
	for i, row := range rows {
		trades[i] = r.toModel(row)
	}

	return trades, nil
}

// ListByUserIDOrderByLatestMessage は、ユーザー userID が申し込んだか申し込まれた交換を、終わったものも含めて、
// 最新のメッセージが新しい順に返す。メッセージの無い交換は申し込まれた時刻で並べる。
func (r *TradeRepository) ListByUserIDOrderByLatestMessage(ctx context.Context, userID model.UserID) ([]*model.Trade, error) {
	rows, err := r.q.ListTradesByUserIDOrderByLatestMessage(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	trades := make([]*model.Trade, len(rows))
	for i, row := range rows {
		trades[i] = r.toModel(row)
	}

	return trades, nil
}

// ListEndedByUserID は、ユーザー userID が申し込んだか申し込まれた、終わった交換を、終わった時刻が新しい順に返す。
func (r *TradeRepository) ListEndedByUserID(ctx context.Context, userID model.UserID) ([]*model.Trade, error) {
	rows, err := r.q.ListEndedTradesByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	trades := make([]*model.Trade, len(rows))
	for i, row := range rows {
		trades[i] = r.toModel(row)
	}

	return trades, nil
}

// CountEndedByUserIDGroupByStatus は、ユーザー userID が申し込んだか申し込まれた、終わった交換の数を段階ごとに返す。
// 1件も無い段階はmapに入れない。
func (r *TradeRepository) CountEndedByUserIDGroupByStatus(ctx context.Context, userID model.UserID) (map[model.TradeStatus]int64, error) {
	rows, err := r.q.CountEndedTradesByUserIDGroupByStatus(ctx, uuid.UUID(userID))
	if err != nil {
		return nil, err
	}

	counts := make(map[model.TradeStatus]int64, len(rows))
	for _, row := range rows {
		counts[model.TradeStatus(row.Status)] = row.Count
	}

	return counts, nil
}

// CountAwaitingByUserID は、ユーザー userID の返事や確認を待っている交換の数を返す。
// 数える条件は model.Trade.IsAwaiting と同じ。
func (r *TradeRepository) CountAwaitingByUserID(ctx context.Context, userID model.UserID) (int64, error) {
	return r.q.CountAwaitingTradesByUserID(ctx, uuid.UUID(userID))
}

// Withdraw は、ユーザー proposerUserID が申し込んだ返事待ちの交換 id を取り下げる。
// 返事待ちでなくなっていた (承認・お断り・取り下げが先に済んだ) か、申し込んだ人が違うときは取り下げず、falseを返す。
func (r *TradeRepository) Withdraw(ctx context.Context, id model.TradeID, proposerUserID model.UserID) (bool, error) {
	affected, err := r.q.WithdrawTrade(ctx, query.WithdrawTradeParams{
		ID:             uuid.UUID(id),
		ProposerUserID: uuid.UUID(proposerUserID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Approve は、ユーザー receiverUserID が申し込まれた返事待ちの交換 id を承認し、マッチ成立にする。
// 返事待ちでなくなっていた (取り下げ・お断り・承認が先に済んだ) か、申し込まれた人が違うときは承認せず、falseを返す。
func (r *TradeRepository) Approve(ctx context.Context, id model.TradeID, receiverUserID model.UserID) (bool, error) {
	affected, err := r.q.ApproveTrade(ctx, query.ApproveTradeParams{
		ID:             uuid.UUID(id),
		ReceiverUserID: uuid.UUID(receiverUserID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Decline は、ユーザー receiverUserID が申し込まれた返事待ちの交換 id をお断りする。
// 返事待ちでなくなっていた (取り下げ・承認・お断りが先に済んだ) か、申し込まれた人が違うときはお断りせず、falseを返す。
func (r *TradeRepository) Decline(ctx context.Context, id model.TradeID, receiverUserID model.UserID) (bool, error) {
	affected, err := r.q.DeclineTrade(ctx, query.DeclineTradeParams{
		ID:             uuid.UUID(id),
		ReceiverUserID: uuid.UUID(receiverUserID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Complete は、交換の2人のうちユーザー userID が、マッチ成立の交換 id で「交換できた」を押したことを記録し、更新した交換を返す。
// 相手がすでに押していれば、同じ更新で交換を「交換できた」の段階で終える。
// マッチ成立でなくなっていた (相手が先にやめた・交換できなかったを記録した) か、すでに押していたか、交換の2人でないときは記録せず、nilを返す。
func (r *TradeRepository) Complete(ctx context.Context, id model.TradeID, userID model.UserID) (*model.Trade, error) {
	row, err := r.q.CompleteTrade(ctx, query.CompleteTradeParams{
		ID:     uuid.UUID(id),
		UserID: uuid.UUID(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// Fail は、交換の2人のうちユーザー userID が、マッチ成立の交換 id を「交換できなかった」の段階で終える。
// マッチ成立でなくなっていた (相手が先にやめた・交換できなかったを記録した・2人そろって交換できた) か、交換の2人でないときは終えず、falseを返す。
func (r *TradeRepository) Fail(ctx context.Context, id model.TradeID, userID model.UserID) (bool, error) {
	affected, err := r.q.FailTrade(ctx, query.FailTradeParams{
		ID:     uuid.UUID(id),
		UserID: uuid.UUID(userID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// Cancel は、交換の2人のうちユーザー userID が、マッチ成立の交換 id をやめる。
// マッチ成立でなくなっていたか、どちらかが「交換できた」を押していたか、交換の2人でないときはやめず、falseを返す。
func (r *TradeRepository) Cancel(ctx context.Context, id model.TradeID, userID model.UserID) (bool, error) {
	affected, err := r.q.CancelTrade(ctx, query.CancelTradeParams{
		ID:     uuid.UUID(id),
		UserID: uuid.UUID(userID),
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModel はクエリの行を model.Trade に変換する。
func (r *TradeRepository) toModel(row query.Trade) *model.Trade {
	return &model.Trade{
		ID:                  model.TradeID(row.ID),
		ProposerUserID:      model.UserID(row.ProposerUserID),
		ReceiverUserID:      model.UserID(row.ReceiverUserID),
		Status:              model.TradeStatus(row.Status),
		ProposerCompletedAt: row.ProposerCompletedAt,
		ReceiverCompletedAt: row.ReceiverCompletedAt,
		MatchedAt:           row.MatchedAt,
		EndedAt:             row.EndedAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}
