package validator

import "context"

// EventCategoryArchiveCreateValidator は管理画面のカテゴリーのアーカイブのフォームを検証する。
// あとから見た運営が経緯を追えるよう、理由を必須にする。
type EventCategoryArchiveCreateValidator struct{}

// NewEventCategoryArchiveCreateValidator は EventCategoryArchiveCreateValidator を生成する。
func NewEventCategoryArchiveCreateValidator() *EventCategoryArchiveCreateValidator {
	return &EventCategoryArchiveCreateValidator{}
}

// EventCategoryArchiveCreateValidatorInput は EventCategoryArchiveCreateValidator.Validate の入力。
type EventCategoryArchiveCreateValidatorInput struct {
	ArchiveMessage string
}

// Validate はアーカイブの理由を検証し、前後の空白を除いた理由を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventCategoryArchiveCreateValidator) Validate(ctx context.Context, input EventCategoryArchiveCreateValidatorInput) (string, error) {
	return validateArchiveMessage(ctx, input.ArchiveMessage)
}
