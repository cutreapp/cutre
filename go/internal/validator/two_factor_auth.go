package validator

import (
	"context"
	"regexp"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// totpCodePattern はTOTPのコードの形式 (半角数字6桁)。
var totpCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// TwoFactorAuthCreateValidator は、二要素認証を有効にするフォームのコードを検証する。
//
// コードの照合は行わない。秘密鍵の復号と、使ったタイムステップの記録が要るため、UseCaseが行う。
type TwoFactorAuthCreateValidator struct{}

// NewTwoFactorAuthCreateValidator は TwoFactorAuthCreateValidator を生成する。
func NewTwoFactorAuthCreateValidator() *TwoFactorAuthCreateValidator {
	return &TwoFactorAuthCreateValidator{}
}

// TwoFactorAuthCreateValidatorInput は TwoFactorAuthCreateValidator.Validate の入力。
type TwoFactorAuthCreateValidatorInput struct {
	// Code は auth.NormalizeTOTPCode で空白とハイフンを取り除いたコード。
	Code string
}

// Validate はコードが入力され、半角数字6桁であることを確かめる。
func (v *TwoFactorAuthCreateValidator) Validate(ctx context.Context, input TwoFactorAuthCreateValidatorInput) error {
	ve := model.NewValidationError()
	addTOTPCodeErrors(ctx, ve, input.Code)
	if ve.HasErrors() {
		return ve
	}

	return nil
}

// addTOTPCodeErrors は認証アプリのコードが入力され、半角数字6桁であることを確かめ、誤りをコードの欄に加える。
// 有効にするときとログインするときで、同じ形のコードを受け付けるため共有する。
func addTOTPCodeErrors(ctx context.Context, ve *model.ValidationError, code string) {
	switch {
	case code == "":
		ve.AddField("code", i18n.T(ctx, "validation_required"))
	case !totpCodePattern.MatchString(code):
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_format"))
	}
}

// TwoFactorAuthDeleteValidator は、二要素認証を無効にするフォームの再認証の入力を検証する。
//
// 入力はパスワードか認証アプリのコードのどちらかで、照合は行わない。
// コードとパスワードの両方を試す照合には、TOTPの秘密鍵の復号とパスワードのハッシュが要るため、UseCaseが行う。
type TwoFactorAuthDeleteValidator struct{}

// NewTwoFactorAuthDeleteValidator は TwoFactorAuthDeleteValidator を生成する。
func NewTwoFactorAuthDeleteValidator() *TwoFactorAuthDeleteValidator {
	return &TwoFactorAuthDeleteValidator{}
}

// TwoFactorAuthDeleteValidatorInput は TwoFactorAuthDeleteValidator.Validate の入力。
type TwoFactorAuthDeleteValidatorInput struct {
	// Credential は入力されたままのパスワードまたは認証アプリのコード。
	Credential string
}

// Validate はパスワードまたはコードが入力されていることを確かめる。
//
// 形式は確かめない。パスワードの長さのポリシーを確かめない理由はログインと同じで、
// 6桁の数字でない入力もパスワードとして照合するため。
func (v *TwoFactorAuthDeleteValidator) Validate(ctx context.Context, input TwoFactorAuthDeleteValidatorInput) error {
	if input.Credential == "" {
		ve := model.NewValidationError()
		ve.AddField("credential", i18n.T(ctx, "validation_required"))
		return ve
	}

	return nil
}
