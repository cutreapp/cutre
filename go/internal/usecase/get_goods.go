package usecase

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetGoodsUsecase はユーザーがリストに入れるグッズを、そのカテゴリーとイベントと一緒に引く。
type GetGoodsUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewGetGoodsUsecase は GetGoodsUsecase を生成する。
func NewGetGoodsUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *GetGoodsUsecase {
	return &GetGoodsUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// GetGoodsInput は GetGoodsUsecase.Execute の入力。
type GetGoodsInput struct {
	GoodsID model.GoodsID
}

// GetGoodsOutput は GetGoodsUsecase.Execute の結果。
type GetGoodsOutput struct {
	Goods         *model.Goods
	EventCategory *model.EventCategory
	Event         *model.Event
}

// Execute は公開中のグッズと、そのカテゴリーとイベントを返す。
// グッズが無いか、グッズ・カテゴリー・イベントのいずれかが公開中でないときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetGoodsUsecase) Execute(ctx context.Context, input GetGoodsInput) (*GetGoodsOutput, error) {
	goods, category, event, err := findPublishedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID)
	if err != nil {
		return nil, err
	}

	return &GetGoodsOutput{Goods: goods, EventCategory: category, Event: event}, nil
}
