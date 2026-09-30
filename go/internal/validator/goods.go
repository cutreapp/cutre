package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GoodsCreateValidator は管理画面のグッズの作成のフォームを検証する。
type GoodsCreateValidator struct{}

// NewGoodsCreateValidator は GoodsCreateValidator を生成する。
func NewGoodsCreateValidator() *GoodsCreateValidator {
	return &GoodsCreateValidator{}
}

// GoodsCreateValidatorInput は GoodsCreateValidator.Validate の入力。フォームの値をそのまま受け取る。
type GoodsCreateValidatorInput struct {
	Name     string
	Position string
}

// Validate はグッズの作成のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *GoodsCreateValidator) Validate(ctx context.Context, input GoodsCreateValidatorInput) (*repository.GoodsAttributes, error) {
	return validateGoodsAttributes(ctx, input.Name, input.Position)
}

// GoodsUpdateValidator は管理画面のグッズの編集のフォームを検証する。
// 検証する項目は作成と同じで、版の競合はUseCaseが更新のときに確かめる。
type GoodsUpdateValidator struct{}

// NewGoodsUpdateValidator は GoodsUpdateValidator を生成する。
func NewGoodsUpdateValidator() *GoodsUpdateValidator {
	return &GoodsUpdateValidator{}
}

// GoodsUpdateValidatorInput は GoodsUpdateValidator.Validate の入力。フォームの値をそのまま受け取る。
type GoodsUpdateValidatorInput struct {
	Name     string
	Position string
}

// Validate はグッズの編集のフォームを検証し、保存する属性を返す。
// 入力の誤りは *model.ValidationError で返す。
func (v *GoodsUpdateValidator) Validate(ctx context.Context, input GoodsUpdateValidatorInput) (*repository.GoodsAttributes, error) {
	return validateGoodsAttributes(ctx, input.Name, input.Position)
}

// validateGoodsAttributes はグッズの名前と並び順を検証し、保存する属性に変換する。
func validateGoodsAttributes(ctx context.Context, name, position string) (*repository.GoodsAttributes, error) {
	ve := model.NewValidationError()
	validName, validPosition := validateMasterNameAndPosition(ctx, ve, name, position)
	if ve.HasErrors() {
		return nil, ve
	}

	return &repository.GoodsAttributes{Name: validName, Position: validPosition}, nil
}
