package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// UnarchiveEventUsecase は管理画面でアーカイブしたイベントを公開に戻す。
type UnarchiveEventUsecase struct {
	eventRepo *repository.EventRepository
}

// NewUnarchiveEventUsecase は UnarchiveEventUsecase を生成する。
func NewUnarchiveEventUsecase(eventRepo *repository.EventRepository) *UnarchiveEventUsecase {
	return &UnarchiveEventUsecase{eventRepo: eventRepo}
}

// UnarchiveEventInput は UnarchiveEventUsecase.Execute の入力。
type UnarchiveEventInput struct {
	User        *model.User
	EventID     model.EventID
	LockVersion int32
}

// Execute はイベントを公開に戻して理由を空にし、戻したユーザーとイベントをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、イベントが無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// アーカイブしていなかったときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *UnarchiveEventUsecase) Execute(ctx context.Context, input UnarchiveEventInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, err := findUndeletedEvent(ctx, uc.eventRepo, input.EventID); err != nil {
		return err
	}

	unarchived, err := uc.eventRepo.Unarchive(ctx, input.EventID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("イベントを元に戻すのに失敗: %w", err)
	}
	if !unarchived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_id": input.EventID.String()}}
	}
	slog.InfoContext(ctx, "イベントを元に戻しました", "user_id", input.User.ID, "event_id", input.EventID)

	return nil
}
