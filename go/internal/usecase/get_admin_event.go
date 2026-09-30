package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminEventUsecase は管理画面で編集・アーカイブするイベントを、配下のカテゴリーと一緒に引く。
type GetAdminEventUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewGetAdminEventUsecase は GetAdminEventUsecase を生成する。
func NewGetAdminEventUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *GetAdminEventUsecase {
	return &GetAdminEventUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// GetAdminEventInput は GetAdminEventUsecase.Execute の入力。
type GetAdminEventInput struct {
	User    *model.User
	EventID model.EventID
}

// GetAdminEventOutput は GetAdminEventUsecase.Execute の結果。
type GetAdminEventOutput struct {
	Event *model.Event
	// EventCategories はイベントの削除していないカテゴリーを並び順に並べたもの。
	EventCategories []*model.EventCategory
	// CanDelete はユーザーがこのイベントを削除できるか。削除の欄を出すかを決める。
	CanDelete bool
}

// Execute はイベントとそのカテゴリーを返す。1つのイベントのカテゴリーは多くないため、ページに分けずにすべて返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、
// イベントが無いか削除したときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetAdminEventUsecase) Execute(ctx context.Context, input GetAdminEventInput) (*GetAdminEventOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	event, err := findUndeletedEvent(ctx, uc.eventRepo, input.EventID)
	if err != nil {
		return nil, err
	}

	categories, err := uc.eventCategoryRepo.ListUndeletedByEventID(ctx, event.ID)
	if err != nil {
		return nil, fmt.Errorf("イベントのカテゴリーの取得に失敗: %w", err)
	}

	return &GetAdminEventOutput{
		Event:           event,
		EventCategories: categories,
		CanDelete:       policy.NewAdminPolicy(input.User).CanDeleteMaster(),
	}, nil
}
