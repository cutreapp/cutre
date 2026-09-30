package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// ArchiveEventCategoryUsecase は管理画面で公開中のカテゴリーを、理由を残してアーカイブする。
type ArchiveEventCategoryUsecase struct {
	validator         *validator.EventCategoryArchiveCreateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewArchiveEventCategoryUsecase は ArchiveEventCategoryUsecase を生成する。
func NewArchiveEventCategoryUsecase(validator *validator.EventCategoryArchiveCreateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *ArchiveEventCategoryUsecase {
	return &ArchiveEventCategoryUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// ArchiveEventCategoryInput は ArchiveEventCategoryUsecase.Execute の入力。
type ArchiveEventCategoryInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
	LockVersion     int32
	ArchiveMessage  string
}

// Execute はカテゴリーをアーカイブし、アーカイブしたユーザーとカテゴリーをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、カテゴリーが無いか、カテゴリーかイベントを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、理由の誤りは *model.ValidationError を返す。
// 既にアーカイブしていたときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *ArchiveEventCategoryUsecase) Execute(ctx context.Context, input ArchiveEventCategoryInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	category, _, err := findUndeletedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID)
	if err != nil {
		return err
	}
	if category.IsArchived() {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_category_id": input.EventCategoryID.String()}}
	}

	message, err := uc.validator.Validate(ctx, validator.EventCategoryArchiveCreateValidatorInput{ArchiveMessage: input.ArchiveMessage})
	if err != nil {
		return err
	}

	archived, err := uc.eventCategoryRepo.Archive(ctx, input.EventCategoryID, input.LockVersion, message)
	if err != nil {
		return fmt.Errorf("カテゴリーのアーカイブに失敗: %w", err)
	}
	if !archived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_category_id": input.EventCategoryID.String()}}
	}
	slog.InfoContext(ctx, "カテゴリーをアーカイブしました", "user_id", input.User.ID, "event_category_id", input.EventCategoryID)

	return nil
}
