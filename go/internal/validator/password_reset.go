package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// PasswordResetCreateValidator はパスワードリセットの申請のフォーム (メールアドレスの入力) を検証する。
type PasswordResetCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewPasswordResetCreateValidator は PasswordResetCreateValidator を生成する。
func NewPasswordResetCreateValidator(userRepo *repository.UserRepository) *PasswordResetCreateValidator {
	return &PasswordResetCreateValidator{userRepo: userRepo}
}

// PasswordResetCreateValidatorInput は PasswordResetCreateValidator.Validate の入力。
type PasswordResetCreateValidatorInput struct {
	Email string
}

// Validate はメールアドレスの形式を検証し、そのアドレスで登録済みのユーザーを返す (未登録ならnil)。
//
// 未登録のアドレスをフォームのエラーにしない理由は、登録を始めるフォームと同じ (登録の有無を調べる手段にしない)。
// 退会したユーザーは FindByEmail が返さないため、未登録として扱われる。
func (v *PasswordResetCreateValidator) Validate(ctx context.Context, input PasswordResetCreateValidatorInput) (*model.User, error) {
	ve := model.NewValidationError()
	validateEmail(ctx, ve, input.Email)
	if ve.HasErrors() {
		return nil, ve
	}

	return v.userRepo.FindByEmail(ctx, input.Email)
}
