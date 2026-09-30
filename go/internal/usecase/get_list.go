package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetListUsecase はユーザーの譲れる・ほしいのリストの1つを、アイテムのグッズ・カテゴリー・イベントと一緒に引く。
type GetListUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
}

// NewGetListUsecase は GetListUsecase を生成する。
func NewGetListUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *GetListUsecase {
	return &GetListUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// GetListInput は GetListUsecase.Execute の入力。
type GetListInput struct {
	UserID model.UserID
	Kind   model.ItemKind
}

// GetListOutput は GetListUsecase.Execute の結果。
type GetListOutput struct {
	// Items はリスト Kind にあるアイテム。イベントを開始日の新しい順に、イベントの中はカテゴリー・グッズの並び順に並べる。
	Items []*model.Item
	// Goods・EventCategories・Events は、Items のグッズと、そのカテゴリー・イベント。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
	Events          map[model.EventID]*model.Event
	// Quantities は、譲れる・ほしいのリストそれぞれのアイテムの数量の合計。リストを切り替えるボタンに出す。
	Quantities model.ItemQuantities
}

// Execute はリスト Kind にあるアイテムと、そのグッズ・カテゴリー・イベント、リストごとの数量の合計を返す。
//
// アイテムのマスタは段ごとに1回のクエリでまとめて引き、アイテムの数によらずクエリの回数を一定にする。
// 招待制のあいだはアイテムが多くないため、ページに分けずにすべて返す。
func (uc *GetListUsecase) Execute(ctx context.Context, input GetListInput) (*GetListOutput, error) {
	items, err := uc.itemRepo.ListListedByUserIDAndKind(ctx, input.UserID, input.Kind)
	if err != nil {
		return nil, fmt.Errorf("リストのアイテムの取得に失敗: %w", err)
	}

	goodsIDs := make([]model.GoodsID, len(items))
	for i, item := range items {
		goodsIDs[i] = item.GoodsID
	}
	goodsList, err := uc.goodsRepo.ListByIDs(ctx, goodsIDs)
	if err != nil {
		return nil, fmt.Errorf("リストのアイテムのグッズの取得に失敗: %w", err)
	}
	goods := make(map[model.GoodsID]*model.Goods, len(goodsList))
	categoryIDs := make([]model.EventCategoryID, 0, len(goodsList))
	for _, g := range goodsList {
		goods[g.ID] = g
		categoryIDs = append(categoryIDs, g.EventCategoryID)
	}

	categoryList, err := uc.eventCategoryRepo.ListByIDs(ctx, categoryIDs)
	if err != nil {
		return nil, fmt.Errorf("リストのアイテムのカテゴリーの取得に失敗: %w", err)
	}
	categories := make(map[model.EventCategoryID]*model.EventCategory, len(categoryList))
	eventIDs := make([]model.EventID, 0, len(categoryList))
	for _, category := range categoryList {
		categories[category.ID] = category
		eventIDs = append(eventIDs, category.EventID)
	}

	eventList, err := uc.eventRepo.ListByIDs(ctx, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("リストのアイテムのイベントの取得に失敗: %w", err)
	}
	events := make(map[model.EventID]*model.Event, len(eventList))
	for _, event := range eventList {
		events[event.ID] = event
	}

	quantities, err := uc.itemRepo.SumListedQuantitiesByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("リストごとのアイテムの数量の取得に失敗: %w", err)
	}

	return &GetListOutput{Items: items, Goods: goods, EventCategories: categories, Events: events, Quantities: quantities}, nil
}
