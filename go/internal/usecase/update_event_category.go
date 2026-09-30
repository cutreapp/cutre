package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdateEventCategoryUsecase は管理画面でカテゴリーの名前と並び順を更新する。
type UpdateEventCategoryUsecase struct {
	validator         *validator.EventCategoryUpdateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewUpdateEventCategoryUsecase は UpdateEventCategoryUsecase を生成する。
func NewUpdateEventCategoryUsecase(validator *validator.EventCategoryUpdateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *UpdateEventCategoryUsecase {
	return &UpdateEventCategoryUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// UpdateEventCategoryInput は UpdateEventCategoryUsecase.Execute の入力。
type UpdateEventCategoryInput struct {
	User            *model.User
	EventCategoryID model.EventCategoryID
	// LockVersion は編集のフォームを開いたときのカテゴリーの版。
	LockVersion int32
	Name        string
	Position    string
}

// UpdateEventCategoryOutput は UpdateEventCategoryUsecase.Execute の結果。
type UpdateEventCategoryOutput struct {
	// EventID はカテゴリーのイベントのID。ハンドラーが戻り先を決めるのに使う。
	EventID model.EventID
}

// Execute はカテゴリーを更新し、更新したユーザーとカテゴリーをログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、カテゴリーが無いか、カテゴリーかイベントを削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
// フォームを開いたあとにほかの操作で先に更新されていた (版が一致しない) ときは、上書きせずに
// AppErrCodeConflict の *model.AppError を返す。
func (uc *UpdateEventCategoryUsecase) Execute(ctx context.Context, input UpdateEventCategoryInput) (*UpdateEventCategoryOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	category, _, err := findUndeletedEventCategory(ctx, uc.eventRepo, uc.eventCategoryRepo, input.EventCategoryID)
	if err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.EventCategoryUpdateValidatorInput{Name: input.Name, Position: input.Position})
	if err != nil {
		return nil, err
	}

	updated, err := uc.eventCategoryRepo.Update(ctx, input.EventCategoryID, input.LockVersion, *attrs)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーの更新に失敗: %w", err)
	}
	if !updated {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"event_category_id": input.EventCategoryID.String()}}
	}
	slog.InfoContext(ctx, "カテゴリーを更新しました", "user_id", input.User.ID, "event_category_id", input.EventCategoryID)

	return &UpdateEventCategoryOutput{EventID: category.EventID}, nil
}
