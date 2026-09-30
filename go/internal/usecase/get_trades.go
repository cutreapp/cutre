package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTradesUsecase は、交換の画面に出す、ユーザーの進行中の交換を相手と交換の品と一緒に引く。
// これまでの交換への行に添える、終わった交換の段階ごとの数も引く。
type GetTradesUsecase struct {
	itemRepo      *repository.ItemRepository
	tradeRepo     *repository.TradeRepository
	tradeItemRepo *repository.TradeItemRepository
	userRepo      *repository.UserRepository
}

// NewGetTradesUsecase は GetTradesUsecase を生成する。
func NewGetTradesUsecase(
	itemRepo *repository.ItemRepository,
	tradeRepo *repository.TradeRepository,
	tradeItemRepo *repository.TradeItemRepository,
	userRepo *repository.UserRepository,
) *GetTradesUsecase {
	return &GetTradesUsecase{itemRepo: itemRepo, tradeRepo: tradeRepo, tradeItemRepo: tradeItemRepo, userRepo: userRepo}
}

// GetTradesInput は GetTradesUsecase.Execute の入力。
type GetTradesInput struct {
	UserID model.UserID
}

// GetTradesOutput は GetTradesUsecase.Execute の結果。
type GetTradesOutput struct {
	// Trades は進行中 (返事待ち・マッチ成立) の交換で、新しく申し込まれた順に並ぶ。
	Trades []*model.Trade
	// Partners は交換の相手。退会した相手も入る。
	Partners map[model.UserID]*model.User
	// Items は交換ごとの品のアイテムで、入れた順に1点ずつ並ぶ。渡す人はアイテムの持ち主で決まる。
	Items map[model.TradeID][]*model.Item
	// EndedCounts は終わった交換の段階ごとの数。1件も無い段階は入らない。
	EndedCounts map[model.TradeStatus]int64
}

// Execute はユーザーの進行中の交換と、終わった交換の段階ごとの数を返す。
//
// 相手と交換の品は、交換の数によらずクエリの回数を一定にしてまとめて引く。
// 招待制のあいだは件数が多くならないため、ページに分けない。
func (uc *GetTradesUsecase) Execute(ctx context.Context, input GetTradesInput) (*GetTradesOutput, error) {
	trades, err := uc.tradeRepo.ListInProgressByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("進行中の交換の取得に失敗: %w", err)
	}

	partners, items, err := findTradePartnersAndItems(ctx, uc.userRepo, uc.tradeItemRepo, uc.itemRepo, input.UserID, trades)
	if err != nil {
		return nil, err
	}

	endedCounts, err := uc.tradeRepo.CountEndedByUserIDGroupByStatus(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("終わった交換の数の取得に失敗: %w", err)
	}

	return &GetTradesOutput{Trades: trades, Partners: partners, Items: items, EndedCounts: endedCounts}, nil
}
