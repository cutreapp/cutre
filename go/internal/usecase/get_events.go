package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetEventsUsecase はユーザー向けのイベントの一覧 (リストに追加するグッズを探す入口) を引く。
type GetEventsUsecase struct {
	eventRepo *repository.EventRepository
	goodsRepo *repository.GoodsRepository
	itemRepo  *repository.ItemRepository
}

// NewGetEventsUsecase は GetEventsUsecase を生成する。
func NewGetEventsUsecase(eventRepo *repository.EventRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *GetEventsUsecase {
	return &GetEventsUsecase{eventRepo: eventRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// GetEventsInput は GetEventsUsecase.Execute の入力。
type GetEventsInput struct {
	UserID model.UserID
}

// GetEventsOutput は GetEventsUsecase.Execute の結果。
type GetEventsOutput struct {
	// Events は公開中のイベントを開始日の新しい順に並べたもの。
	Events []*model.Event
	// GoodsCounts はイベントごとの公開中のグッズの種類の数。グッズが無いイベントは入らない。
	GoodsCounts map[model.EventID]int64
	// Quantities はイベントごとの、ユーザーのリストにあるアイテムの数量の合計。アイテムが無いイベントは入らない。
	Quantities map[model.EventID]model.ItemQuantities
}

// Execute は公開中のイベントと、イベントごとのグッズの種類の数・ユーザーのアイテムの数量を返す。
// 招待制のあいだはイベントが多くないため、ページに分けずにすべて返す。
func (uc *GetEventsUsecase) Execute(ctx context.Context, input GetEventsInput) (*GetEventsOutput, error) {
	events, err := uc.eventRepo.ListPublished(ctx)
	if err != nil {
		return nil, fmt.Errorf("イベントの一覧の取得に失敗: %w", err)
	}

	goodsCounts, err := uc.goodsRepo.CountPublishedGroupByEventID(ctx)
	if err != nil {
		return nil, fmt.Errorf("イベントごとのグッズの数の取得に失敗: %w", err)
	}

	quantities, err := uc.itemRepo.SumListedQuantitiesByUserIDGroupByEventID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("イベントごとのアイテムの数量の取得に失敗: %w", err)
	}

	return &GetEventsOutput{Events: events, GoodsCounts: goodsCounts, Quantities: quantities}, nil
}
