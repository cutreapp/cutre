package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// SignUpCreateValidator は登録を始めるフォーム (メールアドレスの入力) を検証する。
type SignUpCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewSignUpCreateValidator は SignUpCreateValidator を生成する。
func NewSignUpCreateValidator(userRepo *repository.UserRepository) *SignUpCreateValidator {
	return &SignUpCreateValidator{userRepo: userRepo}
}

// SignUpCreateValidatorInput は SignUpCreateValidator.Validate の入力。
type SignUpCreateValidatorInput struct {
	Email string
}

// Validate はメールアドレスの形式を検証し、そのアドレスで登録済みのユーザーを返す (未登録ならnil)。
//
// 登録済みのアドレスをフォームのエラーにしない。画面で知らせると登録の有無を調べる手段になるため、
// 呼び出し側は未登録のときと同じ画面を出し、メールでログインを案内する。
// 退会したユーザーは FindByEmail が返さないため、未登録として扱われる (解放されたアドレスは再び登録できる)。
func (v *SignUpCreateValidator) Validate(ctx context.Context, input SignUpCreateValidatorInput) (*model.User, error) {
	ve := model.NewValidationError()
	validateEmail(ctx, ve, input.Email)
	if ve.HasErrors() {
		return nil, ve
	}

	return v.userRepo.FindByEmail(ctx, input.Email)
}
