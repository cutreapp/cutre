package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminEventsUsecase は管理画面のイベントの一覧を引く。
type GetAdminEventsUsecase struct {
	eventRepo *repository.EventRepository
}

// NewGetAdminEventsUsecase は GetAdminEventsUsecase を生成する。
func NewGetAdminEventsUsecase(eventRepo *repository.EventRepository) *GetAdminEventsUsecase {
	return &GetAdminEventsUsecase{eventRepo: eventRepo}
}

// GetAdminEventsInput は GetAdminEventsUsecase.Execute の入力。
type GetAdminEventsInput struct {
	User *model.User
}

// GetAdminEventsOutput は GetAdminEventsUsecase.Execute の結果。
type GetAdminEventsOutput struct {
	// Events は削除していないイベントを、開始日の新しい順に並べたもの。
	Events []*model.Event
}

// Execute は削除していないイベントを返す。
// 運営が用意するイベントは多くないため、ページに分けずにすべて返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の *model.AppError を返す。
func (uc *GetAdminEventsUsecase) Execute(ctx context.Context, input GetAdminEventsInput) (*GetAdminEventsOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	events, err := uc.eventRepo.ListUndeleted(ctx)
	if err != nil {
		return nil, fmt.Errorf("イベントの一覧の取得に失敗: %w", err)
	}

	return &GetAdminEventsOutput{Events: events}, nil
}
