package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateEventCategoryUsecase は管理画面でイベントの配下にカテゴリーを作成する。
type CreateEventCategoryUsecase struct {
	validator         *validator.EventCategoryCreateValidator
	eventRepo         *repository.EventRepository
	eventCategoryRepo *repository.EventCategoryRepository
}

// NewCreateEventCategoryUsecase は CreateEventCategoryUsecase を生成する。
func NewCreateEventCategoryUsecase(validator *validator.EventCategoryCreateValidator, eventRepo *repository.EventRepository, eventCategoryRepo *repository.EventCategoryRepository) *CreateEventCategoryUsecase {
	return &CreateEventCategoryUsecase{validator: validator, eventRepo: eventRepo, eventCategoryRepo: eventCategoryRepo}
}

// CreateEventCategoryInput は CreateEventCategoryUsecase.Execute の入力。並び順はフォームの値をそのまま受け取る。
type CreateEventCategoryInput struct {
	User     *model.User
	EventID  model.EventID
	Name     string
	Position string
}

// CreateEventCategoryOutput は CreateEventCategoryUsecase.Execute の結果。
type CreateEventCategoryOutput struct {
	EventCategory *model.EventCategory
}

// Execute は公開中のカテゴリーを作成し、作成したユーザーとカテゴリーをログに残す。
// アーカイブしたイベントにも、公開に戻すときに備えてカテゴリーを作れる。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、イベントが無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
func (uc *CreateEventCategoryUsecase) Execute(ctx context.Context, input CreateEventCategoryInput) (*CreateEventCategoryOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	if _, err := findUndeletedEvent(ctx, uc.eventRepo, input.EventID); err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.EventCategoryCreateValidatorInput{Name: input.Name, Position: input.Position})
	if err != nil {
		return nil, err
	}

	category, err := uc.eventCategoryRepo.Create(ctx, input.EventID, *attrs)
	if err != nil {
		return nil, fmt.Errorf("カテゴリーの作成に失敗: %w", err)
	}
	slog.InfoContext(ctx, "カテゴリーを作成しました", "user_id", input.User.ID, "event_id", input.EventID, "event_category_id", category.ID)

	return &CreateEventCategoryOutput{EventCategory: category}, nil
}
