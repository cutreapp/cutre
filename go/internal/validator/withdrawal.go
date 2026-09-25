package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// WithdrawalDeleteValidator は退会のフォームを検証する。
// 今のパスワードが入力されてユーザーのパスワードと一致し、退会を元に戻せないことを確認したか確かめる。
type WithdrawalDeleteValidator struct {
	userPasswordRepo *repository.UserPasswordRepository
}

// NewWithdrawalDeleteValidator は WithdrawalDeleteValidator を生成する。
func NewWithdrawalDeleteValidator(userPasswordRepo *repository.UserPasswordRepository) *WithdrawalDeleteValidator {
	return &WithdrawalDeleteValidator{userPasswordRepo: userPasswordRepo}
}

// WithdrawalDeleteValidatorInput は WithdrawalDeleteValidator.Validate の入力。
type WithdrawalDeleteValidatorInput struct {
	UserID          model.UserID
	CurrentPassword string
	// Confirmed は退会を元に戻せないことを確認したか。
	Confirmed bool
}

// Validate は退会のフォームを検証する。
// 入力の誤りは *model.ValidationError で、データベースに到達できないなどの失敗は素のerrorで返す。
//
// パスワードの長さのポリシーは確かめない。ログインと同じく、登録済みのパスワードと一致するかだけを見る。
// 二要素認証を有効にしていても、認証アプリのコードは求めない (パスワードで本人であることを確かめる)。
func (v *WithdrawalDeleteValidator) Validate(ctx context.Context, input WithdrawalDeleteValidatorInput) error {
	ve := model.NewValidationError()

	if input.CurrentPassword == "" {
		ve.AddField("current_password", i18n.T(ctx, "validation_required"))
	} else {
		password, err := v.userPasswordRepo.FindByUserID(ctx, input.UserID)
		if err != nil {
			return err
		}
		if password == nil || auth.CheckPassword(password.PasswordDigest, input.CurrentPassword) != nil {
			ve.AddField("current_password", i18n.T(ctx, "validation_current_password_incorrect"))
		}
	}
	if !input.Confirmed {
		ve.AddField("confirmed", i18n.T(ctx, "validation_withdrawal_unconfirmed"))
	}

	if ve.HasErrors() {
		return ve
	}

	return nil
}
