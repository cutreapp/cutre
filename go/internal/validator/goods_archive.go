package validator

import "context"

// GoodsArchiveCreateValidator は管理画面のグッズのアーカイブのフォームを検証する。
// あとから見た運営が経緯を追えるよう、理由を必須にする。
type GoodsArchiveCreateValidator struct{}

// NewGoodsArchiveCreateValidator は GoodsArchiveCreateValidator を生成する。
func NewGoodsArchiveCreateValidator() *GoodsArchiveCreateValidator {
	return &GoodsArchiveCreateValidator{}
}

// GoodsArchiveCreateValidatorInput は GoodsArchiveCreateValidator.Validate の入力。
type GoodsArchiveCreateValidatorInput struct {
	ArchiveMessage string
}

// Validate はアーカイブの理由を検証し、前後の空白を除いた理由を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *GoodsArchiveCreateValidator) Validate(ctx context.Context, input GoodsArchiveCreateValidatorInput) (string, error) {
	return validateArchiveMessage(ctx, input.ArchiveMessage)
}
