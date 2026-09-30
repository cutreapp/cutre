package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTradeHistoryUsecase は、これまでの交換の画面に出す、ユーザーの終わった交換を相手と交換の品と一緒に引く。
type GetTradeHistoryUsecase struct {
	itemRepo      *repository.ItemRepository
	tradeRepo     *repository.TradeRepository
	tradeItemRepo *repository.TradeItemRepository
	userRepo      *repository.UserRepository
}

// NewGetTradeHistoryUsecase は GetTradeHistoryUsecase を生成する。
func NewGetTradeHistoryUsecase(
	itemRepo *repository.ItemRepository,
	tradeRepo *repository.TradeRepository,
	tradeItemRepo *repository.TradeItemRepository,
	userRepo *repository.UserRepository,
) *GetTradeHistoryUsecase {
	return &GetTradeHistoryUsecase{itemRepo: itemRepo, tradeRepo: tradeRepo, tradeItemRepo: tradeItemRepo, userRepo: userRepo}
}

// GetTradeHistoryInput は GetTradeHistoryUsecase.Execute の入力。
type GetTradeHistoryInput struct {
	UserID model.UserID
}

// GetTradeHistoryOutput は GetTradeHistoryUsecase.Execute の結果。
type GetTradeHistoryOutput struct {
	// Trades は終わった (取り下げ・お断り・交換できた・交換できなかった・やめた) 交換で、終わった時刻が新しい順に並ぶ。
	Trades []*model.Trade
	// Partners は交換の相手。退会した相手も入る。
	Partners map[model.UserID]*model.User
	// Items は交換ごとの品のアイテムで、入れた順に1点ずつ並ぶ。渡す人はアイテムの持ち主で決まる。
	Items map[model.TradeID][]*model.Item
}

// Execute はユーザーの終わった交換を返す。
//
// 相手と交換の品は、交換の数によらずクエリの回数を一定にしてまとめて引く。
// 招待制のあいだは件数が多くならないため、ページに分けない。
func (uc *GetTradeHistoryUsecase) Execute(ctx context.Context, input GetTradeHistoryInput) (*GetTradeHistoryOutput, error) {
	trades, err := uc.tradeRepo.ListEndedByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("終わった交換の取得に失敗: %w", err)
	}

	partners, items, err := findTradePartnersAndItems(ctx, uc.userRepo, uc.tradeItemRepo, uc.itemRepo, input.UserID, trades)
	if err != nil {
		return nil, err
	}

	return &GetTradeHistoryOutput{Trades: trades, Partners: partners, Items: items}, nil
}
