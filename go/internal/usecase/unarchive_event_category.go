package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// UnarchiveEventCategoryUsecase は管理画面でアーカイブしたカテゴリーを公開に戻す。
type UnarchiveEventCategoryUsecase struct {
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewUnarchiveEventCategoryUsecase は UnarchiveEventCategoryUsecase を生成する。
func NewUnarchiveEventCategoryUsecase(eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *UnarchiveEventCategoryUsecase {
	return &UnarchiveEventCategoryUsecase{eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// UnarchiveEventCategoryInput は UnarchiveEventCategoryUsecase.Execute の入力。
type UnarchiveEventCategoryInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
	LockVersion     int32
}

// Execute はカテゴリーを公開に戻して理由を空にし、戻したユーザーとカテゴリーをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、カテゴリーが無いか、カテゴリーかイベントを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// アーカイブしていなかったときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *UnarchiveEventCategoryUsecase) Execute(ctx context.Context, input UnarchiveEventCategoryInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, _, err := findUndeletedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID); err != nil {
		return err
	}

	unarchived, err := uc.eventCategoryRepo.Unarchive(ctx, input.EventCategoryID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("カテゴリーを元に戻すのに失敗: %w", err)
	}
	if !unarchived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_category_id": input.EventCategoryID.String()}}
	}
	slog.InfoContext(ctx, "カテゴリーを元に戻しました", "user_id", input.User.ID, "event_category_id", input.EventCategoryID)

	return nil
}
