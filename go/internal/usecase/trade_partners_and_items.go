package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findTradePartnersAndItems は、ユーザー viewerID の交換 trades の相手と、交換ごとの品のアイテムを返す。
// 交換の数によらずクエリの回数を一定にしてまとめて引く。相手は退会していても返す。
func findTradePartnersAndItems(
	ctx context.Context,
	userRepo *repository.UserRepository,
	tradeItemRepo *repository.TradeItemRepository,
	itemRepo *repository.ItemRepository,
	viewerID model.UserID,
	trades []*model.Trade,
) (map[model.UserID]*model.User, map[model.TradeID][]*model.Item, error) {
	tradeIDs := make([]model.TradeID, len(trades))
	partnerIDs := make([]model.UserID, len(trades))
	for i, trade := range trades {
		tradeIDs[i] = trade.ID
		partnerIDs[i] = trade.PartnerUserID(viewerID)
	}

	partnerList, err := userRepo.ListByIDs(ctx, partnerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("交換の相手の取得に失敗: %w", err)
	}
	partners := make(map[model.UserID]*model.User, len(partnerList))
	for _, partner := range partnerList {
		partners[partner.ID] = partner
	}

	items, err := findTradeItems(ctx, tradeItemRepo, itemRepo, tradeIDs)
	if err != nil {
		return nil, nil, err
	}

	return partners, items, nil
}
