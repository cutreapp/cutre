package validator

import (
	"context"
	"regexp"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// confirmationCodePattern は確認コードの形式 (半角数字6桁)。
var confirmationCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// EmailConfirmationCreateValidator は確認コードの入力フォームを検証する。
//
// コードの照合は行わない。誤ったコードでは誤入力の回数を数える (データベースを更新する) ため、UseCaseが行う。
type EmailConfirmationCreateValidator struct{}

// NewEmailConfirmationCreateValidator は EmailConfirmationCreateValidator を生成する。
func NewEmailConfirmationCreateValidator() *EmailConfirmationCreateValidator {
	return &EmailConfirmationCreateValidator{}
}

// EmailConfirmationCreateValidatorInput は EmailConfirmationCreateValidator.Validate の入力。
type EmailConfirmationCreateValidatorInput struct {
	Code string
}

// Validate はコードが入力され、半角数字6桁であることを確かめる。
// 形式の誤りは誤入力として数えず、照合の前に返す。打ち間違いで試行の回数を失わないようにするため。
func (v *EmailConfirmationCreateValidator) Validate(ctx context.Context, input EmailConfirmationCreateValidatorInput) error {
	ve := model.NewValidationError()

	switch {
	case input.Code == "":
		ve.AddField("code", i18n.T(ctx, "validation_required"))
	case !confirmationCodePattern.MatchString(input.Code):
		ve.AddField("code", i18n.T(ctx, "validation_confirmation_code_format"))
	}
	if ve.HasErrors() {
		return ve
	}

	return nil
}
