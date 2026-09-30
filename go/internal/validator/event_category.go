package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// EventCategoryCreateValidator は管理画面のカテゴリーの作成のフォームを検証する。
type EventCategoryCreateValidator struct{}

// NewEventCategoryCreateValidator は EventCategoryCreateValidator を生成する。
func NewEventCategoryCreateValidator() *EventCategoryCreateValidator {
	return &EventCategoryCreateValidator{}
}

// EventCategoryCreateValidatorInput は EventCategoryCreateValidator.Validate の入力。フォームの値をそのまま受け取る。
type EventCategoryCreateValidatorInput struct {
	Name     string
	Position string
}

// Validate はカテゴリーの作成のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventCategoryCreateValidator) Validate(ctx context.Context, input EventCategoryCreateValidatorInput) (*repository.EventCategoryAttributes, error) {
	return validateEventCategoryAttributes(ctx, input.Name, input.Position)
}

// EventCategoryUpdateValidator は管理画面のカテゴリーの編集のフォームを検証する。
// 検証する項目は作成と同じで、版の競合はUseCaseが更新のときに確かめる。
type EventCategoryUpdateValidator struct{}

// NewEventCategoryUpdateValidator は EventCategoryUpdateValidator を生成する。
func NewEventCategoryUpdateValidator() *EventCategoryUpdateValidator {
	return &EventCategoryUpdateValidator{}
}

// EventCategoryUpdateValidatorInput は EventCategoryUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type EventCategoryUpdateValidatorInput struct {
	Name     string
	Position string
}

// Validate はカテゴリーの編集のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventCategoryUpdateValidator) Validate(ctx context.Context, input EventCategoryUpdateValidatorInput) (*repository.EventCategoryAttributes, error) {
	return validateEventCategoryAttributes(ctx, input.Name, input.Position)
}

// validateEventCategoryAttributes はカテゴリーの名前と並び順を検証し、保存する属性に変換する。
func validateEventCategoryAttributes(ctx context.Context, name, position string) (*repository.EventCategoryAttributes, error) {
	ve := model.NewValidationError()
	validName, validPosition := validateMasterNameAndPosition(ctx, ve, name, position)
	if ve.HasErrors() {
		return nil, ve
	}

	return &repository.EventCategoryAttributes{Name: validName, Position: validPosition}, nil
}
