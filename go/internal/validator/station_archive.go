package validator

import "context"

// StationArchiveCreateValidator は管理画面の駅のアーカイブのフォームを検証する。
// あとから見た運営が経緯を追えるよう、理由を必須にする。
type StationArchiveCreateValidator struct{}

// NewStationArchiveCreateValidator は StationArchiveCreateValidator を生成する。
func NewStationArchiveCreateValidator() *StationArchiveCreateValidator {
	return &StationArchiveCreateValidator{}
}

// StationArchiveCreateValidatorInput は StationArchiveCreateValidator.Validate の入力。
type StationArchiveCreateValidatorInput struct {
	ArchiveMessage string
}

// Validate はアーカイブの理由を検証し、前後の空白を除いた理由を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *StationArchiveCreateValidator) Validate(ctx context.Context, input StationArchiveCreateValidatorInput) (string, error) {
	return validateArchiveMessage(ctx, input.ArchiveMessage)
}
