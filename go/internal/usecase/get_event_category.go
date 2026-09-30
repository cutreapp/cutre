package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetEventCategoryUsecase はユーザー向けのカテゴリーのグッズの一覧を、ユーザーのリストにあるアイテムと一緒に引く。
type GetEventCategoryUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
}

// NewGetEventCategoryUsecase は GetEventCategoryUsecase を生成する。
func NewGetEventCategoryUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository, itemRepo *repository.ItemRepository) *GetEventCategoryUsecase {
	return &GetEventCategoryUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo, itemRepo: itemRepo}
}

// GetEventCategoryInput は GetEventCategoryUsecase.Execute の入力。
// EventID はURLでカテゴリーの親として示されたイベントで、カテゴリーのイベントと食い違えば存在しないページとして扱う。
type GetEventCategoryInput struct {
	UserID          model.UserID
	EventID         model.EventID
	EventCategoryID model.EventCategoryID
}

// GetEventCategoryOutput は GetEventCategoryUsecase.Execute の結果。
type GetEventCategoryOutput struct {
	Event         *model.Event
	EventCategory *model.EventCategory
	// Goods はカテゴリーの公開中のグッズを並び順に並べたもの。
	Goods []*model.Goods
	// Items はカテゴリーのグッズを指す、ユーザーのリストにあるアイテム。グッズごとに、どのリストに何点入れたかを示すのに使う。
	Items []*model.Item
}

// Execute は公開中のカテゴリーと、そのイベント・公開中のグッズ・ユーザーのリストにあるアイテムを返す。
// カテゴリーが無いか、カテゴリーかイベントが公開中でないか、カテゴリーが EventID のイベントのものでないときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetEventCategoryUsecase) Execute(ctx context.Context, input GetEventCategoryInput) (*GetEventCategoryOutput, error) {
	category, event, err := findPublishedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID)
	if err != nil {
		return nil, err
	}
	if event.ID != input.EventID {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"event_id": input.EventID.String(), "event_category_id": input.EventCategoryID.String()}}
	}

	goods, err := uc.goodsRepo.ListPublishedByEventCategoryID(ctx, category.ID)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーのグッズの取得に失敗: %w", err)
	}

	items, err := uc.itemRepo.ListListedByUserIDAndEventCategoryID(ctx, input.UserID, category.ID)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーのグッズのアイテムの取得に失敗: %w", err)
	}

	return &GetEventCategoryOutput{Event: event, EventCategory: category, Goods: goods, Items: items}, nil
}
