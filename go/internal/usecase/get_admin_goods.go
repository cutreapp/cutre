package usecase

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminGoodsUsecase は管理画面で編集・アーカイブするグッズを、そのカテゴリーとイベントと一緒に引く。
type GetAdminGoodsUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
}

// NewGetAdminGoodsUsecase は GetAdminGoodsUsecase を生成する。
func NewGetAdminGoodsUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository, goodsRepo *repository.GoodsRepository) *GetAdminGoodsUsecase {
	return &GetAdminGoodsUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo, goodsRepo: goodsRepo}
}

// GetAdminGoodsInput は GetAdminGoodsUsecase.Execute の入力。
type GetAdminGoodsInput struct {
	User    *model.User
	GoodsID model.GoodsID
}

// GetAdminGoodsOutput は GetAdminGoodsUsecase.Execute の結果。
type GetAdminGoodsOutput struct {
	Goods *model.Goods
	// EventCategory と Event はグッズのカテゴリーとイベント。パンくずと戻り先に使う。
	EventCategory *model.EventCategory
	Event         *model.Event
	// CanDelete はユーザーがこのグッズを削除できるか。削除の欄を出すかを決める。
	CanDelete bool
}

// Execute はグッズとそのカテゴリー・イベントを返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、グッズが無いか、
// グッズ・カテゴリー・イベントのいずれかを削除したときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetAdminGoodsUsecase) Execute(ctx context.Context, input GetAdminGoodsInput) (*GetAdminGoodsOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	goods, category, event, err := findUndeletedGoods(ctx, uc.eventRepo, uc.eventCategoryRepo, uc.goodsRepo, input.GoodsID)
	if err != nil {
		return nil, err
	}

	return &GetAdminGoodsOutput{
		Goods:         goods,
		EventCategory: category,
		Event:         event,
		CanDelete:     policy.NewAdminPolicy(input.User).CanDeleteMaster(),
	}, nil
}
