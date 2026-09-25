package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// SignInTwoFactorRecoveryCreateValidator は、ログインでリカバリーコードを入力するフォームを検証する。
//
// コードの照合は行わない。ダイジェストの鍵と、コードの消費 (条件付きのUPDATE) が要るため、UseCaseが行う。
type SignInTwoFactorRecoveryCreateValidator struct{}

// NewSignInTwoFactorRecoveryCreateValidator は SignInTwoFactorRecoveryCreateValidator を生成する。
func NewSignInTwoFactorRecoveryCreateValidator() *SignInTwoFactorRecoveryCreateValidator {
	return &SignInTwoFactorRecoveryCreateValidator{}
}

// SignInTwoFactorRecoveryCreateValidatorInput は SignInTwoFactorRecoveryCreateValidator.Validate の入力。
type SignInTwoFactorRecoveryCreateValidatorInput struct {
	// Code は auth.NormalizeRecoveryCode で正規化したコード。
	Code string
}

// Validate はコードが入力され、発行するコードの形であることを確かめる。
// 形の判定は発行側 (auth) と同じ文字と長さで行い、発行したコードを拒まないようにする。
func (v *SignInTwoFactorRecoveryCreateValidator) Validate(ctx context.Context, input SignInTwoFactorRecoveryCreateValidatorInput) error {
	ve := model.NewValidationError()

	switch {
	case input.Code == "":
		ve.AddField("code", i18n.T(ctx, "validation_required"))
	case !auth.IsRecoveryCodeFormat(input.Code):
		ve.AddField("code", i18n.T(ctx, "validation_recovery_code_format"))
	}

	if ve.HasErrors() {
		return ve
	}

	return nil
}
