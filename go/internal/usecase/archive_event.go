package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// ArchiveEventUsecase は管理画面で公開中のイベントを、理由を残してアーカイブする。
type ArchiveEventUsecase struct {
	validator *validator.EventArchiveCreateValidator
	eventRepo *repository.EventRepository
}

// NewArchiveEventUsecase は ArchiveEventUsecase を生成する。
func NewArchiveEventUsecase(validator *validator.EventArchiveCreateValidator, eventRepo *repository.EventRepository) *ArchiveEventUsecase {
	return &ArchiveEventUsecase{validator: validator, eventRepo: eventRepo}
}

// ArchiveEventInput は ArchiveEventUsecase.Execute の入力。
type ArchiveEventInput struct {
	User           *model.User
	EventID        model.EventID
	LockVersion    int32
	ArchiveMessage string
}

// Execute はイベントをアーカイブし、アーカイブしたユーザーとイベントをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、イベントが無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、理由の誤りは *model.ValidationError を返す。
// 既にアーカイブしていたときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *ArchiveEventUsecase) Execute(ctx context.Context, input ArchiveEventInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	event, err := findUndeletedEvent(ctx, uc.eventRepo, input.EventID)
	if err != nil {
		return err
	}
	if event.IsArchived() {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_id": input.EventID.String()}}
	}

	message, err := uc.validator.Validate(ctx, validator.EventArchiveCreateValidatorInput{ArchiveMessage: input.ArchiveMessage})
	if err != nil {
		return err
	}

	archived, err := uc.eventRepo.Archive(ctx, input.EventID, input.LockVersion, message)
	if err != nil {
		return fmt.Errorf("イベントのアーカイブに失敗: %w", err)
	}
	if !archived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_id": input.EventID.String()}}
	}
	slog.InfoContext(ctx, "イベントをアーカイブしました", "user_id", input.User.ID, "event_id", input.EventID)

	return nil
}
