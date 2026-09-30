package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateEventUsecase は管理画面でイベントを作成する。
type CreateEventUsecase struct {
	validator *validator.EventCreateValidator
	eventRepo *repository.EventRepository
}

// NewCreateEventUsecase は CreateEventUsecase を生成する。
func NewCreateEventUsecase(validator *validator.EventCreateValidator, eventRepo *repository.EventRepository) *CreateEventUsecase {
	return &CreateEventUsecase{validator: validator, eventRepo: eventRepo}
}

// CreateEventInput は CreateEventUsecase.Execute の入力。開催期間はフォームの値をそのまま受け取る。
type CreateEventInput struct {
	User     *model.User
	Name     string
	StartsOn string
	EndsOn   string
}

// CreateEventOutput は CreateEventUsecase.Execute の結果。
type CreateEventOutput struct {
	Event *model.Event
}

// Execute は公開中のイベントを作成し、作成したユーザーとイベントをログに残す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の *model.AppError を、
// フォームの誤りは *model.ValidationError を返す。
func (uc *CreateEventUsecase) Execute(ctx context.Context, input CreateEventInput) (*CreateEventOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.EventCreateValidatorInput{
		Name:     input.Name,
		StartsOn: input.StartsOn,
		EndsOn:   input.EndsOn,
	})
	if err != nil {
		return nil, err
	}

	event, err := uc.eventRepo.Create(ctx, *attrs)
	if err != nil {
		return nil, fmt.Errorf("イベントの作成に失敗: %w", err)
	}
	slog.InfoContext(ctx, "イベントを作成しました", "user_id", input.User.ID, "event_id", event.ID)

	return &CreateEventOutput{Event: event}, nil
}
