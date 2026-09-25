package validator

import (
	"context"
	"errors"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// PasswordUpdateValidator はパスワードリセットで新しいパスワードを設定するフォームを検証する。
type PasswordUpdateValidator struct{}

// NewPasswordUpdateValidator は PasswordUpdateValidator を生成する。
func NewPasswordUpdateValidator() *PasswordUpdateValidator {
	return &PasswordUpdateValidator{}
}

// PasswordUpdateValidatorInput は PasswordUpdateValidator.Validate の入力。
type PasswordUpdateValidatorInput struct {
	Password string
}

// Validate は新しいパスワードの形式を検証する。
func (v *PasswordUpdateValidator) Validate(ctx context.Context, input PasswordUpdateValidatorInput) error {
	ve := model.NewValidationError()
	validatePassword(ctx, ve, input.Password)
	if ve.HasErrors() {
		return ve
	}

	return nil
}

// validatePassword は新しく決めるパスワードの形式を検証し、誤りを password のエラーとして記録する。
// アカウントの作成とパスワードリセットのフォームで同じ規則を使う。
func validatePassword(ctx context.Context, ve *model.ValidationError, password string) {
	if password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
		return
	}

	switch err := auth.ValidatePasswordStrength(password); {
	case errors.Is(err, auth.ErrPasswordTooShort):
		ve.AddField("password", i18n.T(ctx, "validation_password_too_short"))
	case errors.Is(err, auth.ErrPasswordTooLong):
		ve.AddField("password", i18n.T(ctx, "validation_password_too_long"))
	}
}
