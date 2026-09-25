package validator

import (
	"context"
	"regexp"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// AtnameMaxLength はアットネームの最大の長さ。
// アットネームはASCIIの文字に限るため、形式を満たす値ではバイト数と文字数が一致する。
const AtnameMaxLength = 20

// atnameRegex はアットネームに使える文字 (半角英数字とアンダースコア)。
var atnameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// AccountCreateValidator はアカウントの作成のフォーム (アットネームとパスワード) を検証する。
type AccountCreateValidator struct {
	userRepo *repository.UserRepository
}

// NewAccountCreateValidator は AccountCreateValidator を生成する。
func NewAccountCreateValidator(userRepo *repository.UserRepository) *AccountCreateValidator {
	return &AccountCreateValidator{userRepo: userRepo}
}

// AccountCreateValidatorInput は AccountCreateValidator.Validate の入力。
type AccountCreateValidatorInput struct {
	// Email は確認を済ませたメールアドレス。フォームの入力ではないため、形式は確かめない。
	Email    string
	Atname   string
	Password string
}

// Validate はアットネームとパスワードの形式を検証してから、アットネームとメールアドレスが空いているかを確かめる。
// 形式を満たさない値ではデータベースを引かない。
//
// メールアドレスは確認を済ませた本人にしか届かないため、登録済みであることをフォームのエラーとして伝えてよい。
// 確認コードの送信の時点では未登録で、その後に同じアドレスで別の登録を済ませた場合に起きる。
func (v *AccountCreateValidator) Validate(ctx context.Context, input AccountCreateValidatorInput) error {
	ve := model.NewValidationError()

	switch {
	case input.Atname == "":
		ve.AddField("atname", i18n.T(ctx, "validation_required"))
	case len(input.Atname) > AtnameMaxLength:
		ve.AddField("atname", i18n.T(ctx, "validation_atname_too_long"))
	case !atnameRegex.MatchString(input.Atname):
		ve.AddField("atname", i18n.T(ctx, "validation_atname_invalid_format"))
	}

	validatePassword(ctx, ve, input.Password)

	if ve.HasErrors() {
		return ve
	}

	takenByAtname, err := v.userRepo.FindByAtname(ctx, input.Atname)
	if err != nil {
		return err
	}
	if takenByAtname != nil {
		ve.AddField("atname", i18n.T(ctx, "validation_atname_taken"))
	}

	takenByEmail, err := v.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return err
	}
	if takenByEmail != nil {
		ve.AddGlobal(i18n.T(ctx, "validation_account_email_taken"))
	}

	if ve.HasErrors() {
		return ve
	}

	return nil
}
