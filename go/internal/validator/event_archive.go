package validator

import "context"

// EventArchiveCreateValidator は管理画面のイベントのアーカイブのフォームを検証する。
// あとから見た運営が経緯を追えるよう、理由を必須にする。
type EventArchiveCreateValidator struct{}

// NewEventArchiveCreateValidator は EventArchiveCreateValidator を生成する。
func NewEventArchiveCreateValidator() *EventArchiveCreateValidator {
	return &EventArchiveCreateValidator{}
}

// EventArchiveCreateValidatorInput は EventArchiveCreateValidator.Validate の入力。
type EventArchiveCreateValidatorInput struct {
	ArchiveMessage string
}

// Validate はアーカイブの理由を検証し、前後の空白を除いた理由を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *EventArchiveCreateValidator) Validate(ctx context.Context, input EventArchiveCreateValidatorInput) (string, error) {
	return validateArchiveMessage(ctx, input.ArchiveMessage)
}
