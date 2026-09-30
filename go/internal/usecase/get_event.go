package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetEventUsecase はユーザー向けのイベントのカテゴリーの一覧を引く。
type GetEventUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
}

// NewGetEventUsecase は GetEventUsecase を生成する。
func NewGetEventUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *GetEventUsecase {
	return &GetEventUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// GetEventInput は GetEventUsecase.Execute の入力。
type GetEventInput struct {
	UserID  model.UserID
	EventID model.EventID
}

// GetEventOutput は GetEventUsecase.Execute の結果。
type GetEventOutput struct {
	Event *model.Event
	// EventCategories はイベントの公開中のカテゴリーを並び順に並べたもの。
	EventCategories []*model.EventCategory
	// GoodsCounts はカテゴリーごとの公開中のグッズの種類の数。グッズが無いカテゴリーは入らない。
	GoodsCounts map[model.EventCategoryID]int64
	// Quantities はカテゴリーごとの、ユーザーのリストにあるアイテムの数量の合計。アイテムが無いカテゴリーは入らない。
	Quantities map[model.EventCategoryID]model.ItemQuantities
}

// Execute は公開中のイベントと、その公開中のカテゴリー・カテゴリーごとのグッズの種類の数・ユーザーのアイテムの数量を返す。
// イベントが無いか公開中でないときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetEventUsecase) Execute(ctx context.Context, input GetEventInput) (*GetEventOutput, error) {
	event, err := findPublishedEvent(ctx, uc.eventRepo, input.EventID)
	if err != nil {
		return nil, err
	}

	categories, err := uc.eventCategoryRepo.ListPublishedByEventID(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("イベントのカテゴリーの取得に失敗: %w", err)
	}

	goodsCounts, err := uc.goodsRepo.CountPublishedByEventIDGroupByEventCategoryID(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーごとのグッズの数の取得に失敗: %w", err)
	}

	quantities, err := uc.itemRepo.SumListedQuantitiesByUserIDAndEventIDGroupByEventCategoryID(ctx, input.UserID, event.ID)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーごとのアイテムの数量の取得に失敗: %w", err)
	}

	return &GetEventOutput{Event: event, EventCategories: categories, GoodsCounts: goodsCounts, Quantities: quantities}, nil
}
