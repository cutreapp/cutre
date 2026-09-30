package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetItemUsecase はユーザーのリストにあるアイテムを、そのグッズ・カテゴリー・イベントと一緒に引く。
type GetItemUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
}

// NewGetItemUsecase は GetItemUsecase を生成する。
func NewGetItemUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *GetItemUsecase {
	return &GetItemUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// GetItemInput は GetItemUsecase.Execute の入力。
type GetItemInput struct {
	UserID model.UserID
	ItemID model.ItemID
}

// GetItemOutput は GetItemUsecase.Execute の結果。
type GetItemOutput struct {
	Item          *model.Item
	Goods         *model.Goods
	EventCategory *model.EventCategory
	Event         *model.Event
}

// Execute はユーザーのリストにあるアイテムと、そのグッズ・カテゴリー・イベントを返す。
//
// リストからの参照ではマスタの状態を問わないため、アーカイブしたグッズのアイテムも返す。
// アイテムが無いか、リストから外したものか、ほかのユーザーのものであるときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetItemUsecase) Execute(ctx context.Context, input GetItemInput) (*GetItemOutput, error) {
	item, err := findListedItem(ctx, uc.itemRepo, input.UserID, input.ItemID)
	if err != nil {
		return nil, err
	}

	// アイテムが参照するマスタは外部キーで消せず、削除 (状態の変更) も参照が無いときに限るため、必ずある。
	goods, err := uc.goodsRepo.FindByID(ctx, item.GoodsID)
	if err != nil || goods == nil {
		return nil, fmt.Errorf("アイテムのグッズの取得に失敗 (goods = %v): %w", goods, err)
	}
	category, err := uc.eventCategoryRepo.FindByID(ctx, goods.EventCategoryID)
	if err != nil || category == nil {
		return nil, fmt.Errorf("アイテムのカテゴリーの取得に失敗 (category = %v): %w", category, err)
	}
	event, err := uc.eventRepo.FindByID(ctx, category.EventID)
	if err != nil || event == nil {
		return nil, fmt.Errorf("アイテムのイベントの取得に失敗 (event = %v): %w", event, err)
	}

	return &GetItemOutput{Item: item, Goods: goods, EventCategory: category, Event: event}, nil
}
