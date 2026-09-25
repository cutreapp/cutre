package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
)

// SignInTwoFactorCreateValidator は、ログインで認証アプリのコードを入力するフォームを検証する。
//
// コードの照合は行わない。秘密鍵の復号と、使ったタイムステップの記録が要るため、UseCaseが行う。
type SignInTwoFactorCreateValidator struct{}

// NewSignInTwoFactorCreateValidator は SignInTwoFactorCreateValidator を生成する。
func NewSignInTwoFactorCreateValidator() *SignInTwoFactorCreateValidator {
	return &SignInTwoFactorCreateValidator{}
}

// SignInTwoFactorCreateValidatorInput は SignInTwoFactorCreateValidator.Validate の入力。
type SignInTwoFactorCreateValidatorInput struct {
	// Code は auth.NormalizeTOTPCode で空白とハイフンを取り除いたコード。
	Code string
}

// Validate はコードが入力され、半角数字6桁であることを確かめる。
func (v *SignInTwoFactorCreateValidator) Validate(ctx context.Context, input SignInTwoFactorCreateValidatorInput) error {
	ve := model.NewValidationError()
	addTOTPCodeErrors(ctx, ve, input.Code)
	if ve.HasErrors() {
		return ve
	}

	return nil
}
