package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdateEventUsecase は管理画面でイベントの名前と開催期間を更新する。
type UpdateEventUsecase struct {
	validator *validator.EventUpdateValidator
	eventRepo *repository.EventRepository
}

// NewUpdateEventUsecase は UpdateEventUsecase を生成する。
func NewUpdateEventUsecase(validator *validator.EventUpdateValidator, eventRepo *repository.EventRepository) *UpdateEventUsecase {
	return &UpdateEventUsecase{validator: validator, eventRepo: eventRepo}
}

// UpdateEventInput は UpdateEventUsecase.Execute の入力。
type UpdateEventInput struct {
	User    *model.User
	EventID model.EventID
	// LockVersion は編集のフォームを開いたときのイベントの版。
	LockVersion int32
	Name        string
	StartsOn    string
	EndsOn      string
}

// Execute はイベントを更新し、更新したユーザーとイベントをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、イベントが無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
// フォームを開いたあとにほかの操作で先に更新されていた (版が一致しない) ときは、上書きせずに
// AppErrCodeConflict の *model.AppError を返す。
func (uc *UpdateEventUsecase) Execute(ctx context.Context, input UpdateEventInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, err := findUndeletedEvent(ctx, uc.eventRepo, input.EventID); err != nil {
		return err
	}

	attrs, err := uc.validator.Validate(ctx, validator.EventUpdateValidatorInput{
		Name:     input.Name,
		StartsOn: input.StartsOn,
		EndsOn:   input.EndsOn,
	})
	if err != nil {
		return err
	}

	updated, err := uc.eventRepo.Update(ctx, input.EventID, input.LockVersion, *attrs)
	if err != nil {
		return fmt.Errorf("イベントの更新に失敗: %w", err)
	}
	if !updated {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_id": input.EventID.String()}}
	}
	slog.InfoContext(ctx, "イベントを更新しました", "user_id", input.User.ID, "event_id", input.EventID)

	return nil
}
