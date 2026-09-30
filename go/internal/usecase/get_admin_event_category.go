package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminEventCategoryUsecase は管理画面で編集・アーカイブするカテゴリーを、そのイベントと配下のグッズと一緒に引く。
type GetAdminEventCategoryUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewGetAdminEventCategoryUsecase は GetAdminEventCategoryUsecase を生成する。
func NewGetAdminEventCategoryUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *GetAdminEventCategoryUsecase {
	return &GetAdminEventCategoryUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// GetAdminEventCategoryInput は GetAdminEventCategoryUsecase.Execute の入力。
type GetAdminEventCategoryInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
}

// GetAdminEventCategoryOutput は GetAdminEventCategoryUsecase.Execute の結果。
type GetAdminEventCategoryOutput struct {
	EventCategory *model.EventCategory
	// Event はカテゴリーのイベント。パンくずと戻り先に使う。
	Event *model.Event
	// Goods はカテゴリーの削除していないグッズを並び順に並べたもの。
	Goods []*model.Goods
	// CanDelete はユーザーがこのカテゴリーを削除できるか。削除の欄を出すかを決める。
	CanDelete bool
}

// Execute はカテゴリーとそのイベント・グッズを返す。
// 1つのカテゴリーのグッズは多くないため、ページに分けずにすべて返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、
// カテゴリーが無いか、カテゴリーかイベントを削除したときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetAdminEventCategoryUsecase) Execute(ctx context.Context, input GetAdminEventCategoryInput) (*GetAdminEventCategoryOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	category, event, err := findUndeletedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID)
	if err != nil {
		return nil, err
	}

	goods, err := uc.goodsRepo.ListUndeletedByEventCategoryID(ctx, category.ID)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーのグッズの取得に失敗: %w", err)
	}

	return &GetAdminEventCategoryOutput{
		EventCategory: category,
		Event:         event,
		Goods:         goods,
		CanDelete:     policy.NewAdminPolicy(input.User).CanDeleteMaster(),
	}, nil
}
