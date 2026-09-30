package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// TradeItemRepository はtrade_items (交換の品) を読み書きする。
type TradeItemRepository struct {
	q *query.Queries
}

// NewTradeItemRepository は TradeItemRepository を生成する。
func NewTradeItemRepository(db *sql.DB) *TradeItemRepository {
	return &TradeItemRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい TradeItemRepository を返す。
func (r *TradeItemRepository) WithTx(tx *sql.Tx) *TradeItemRepository {
	return &TradeItemRepository{q: r.q.WithTx(tx)}
}

// CreateMany は、交換 tradeID の品として、アイテム itemIDs を1点ずつ入れる。itemIDs が空なら何もしない。
func (r *TradeItemRepository) CreateMany(ctx context.Context, tradeID model.TradeID, itemIDs []model.ItemID) error {
	if len(itemIDs) == 0 {
		return nil
	}

	return r.q.CreateTradeItems(ctx, query.CreateTradeItemsParams{
		TradeID: uuid.UUID(tradeID),
		ItemIds: itemUUIDs(itemIDs),
	})
}

// ListItemIDsByTradeIDs は、交換 tradeIDs ごとの交換の品のアイテムのIDを、入れた順に返す。
// 同じアイテムを2点入れた交換では、同じIDが2回並ぶ。tradeIDs が空ならクエリを発行せずにnilを返す。
func (r *TradeItemRepository) ListItemIDsByTradeIDs(ctx context.Context, tradeIDs []model.TradeID) (map[model.TradeID][]model.ItemID, error) {
	if len(tradeIDs) == 0 {
		return nil, nil
	}

	uuids := make([]uuid.UUID, len(tradeIDs))
	for i, id := range tradeIDs {
		uuids[i] = uuid.UUID(id)
	}
	rows, err := r.q.ListTradeItemsByTradeIDs(ctx, uuids)
	if err != nil {
		return nil, err
	}

	itemIDs := make(map[model.TradeID][]model.ItemID, len(tradeIDs))
	for _, row := range rows {
		tradeID := model.TradeID(row.TradeID)
		itemIDs[tradeID] = append(itemIDs[tradeID], model.ItemID(row.ItemID))
	}

	return itemIDs, nil
}
