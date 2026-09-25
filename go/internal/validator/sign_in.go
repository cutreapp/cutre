// Package validator はフォームの入力を検証する。
// 形式の検証と、データベースを引いて行う状態の検証の両方を担い、UseCaseから呼び出される。
package validator

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// SignInCreateValidator はログインのフォームを検証する。
// メールアドレスとパスワードが入力され、それらが一致するアカウントを指していることを確かめる。
type SignInCreateValidator struct {
	userRepo         *repository.UserRepository
	userPasswordRepo *repository.UserPasswordRepository
}

// NewSignInCreateValidator は SignInCreateValidator を生成する。
func NewSignInCreateValidator(
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *SignInCreateValidator {
	return &SignInCreateValidator{
		userRepo:         userRepo,
		userPasswordRepo: userPasswordRepo,
	}
}

// SignInCreateValidatorInput は SignInCreateValidator.Validate の入力。
type SignInCreateValidatorInput struct {
	Email    string
	Password string
}

// Validate は送信された資格情報を検証し、一致したユーザーを返す。
// 入力の誤りは *model.ValidationError で、データベースに到達できないなどの失敗は素のerrorで返す。
//
// 資格情報の不一致 (未知のメールアドレス・パスワードを持たないアカウント・誤ったパスワード) は、
// どれに当たるかを明かさない1つのメッセージで伝える。
// 区別すると、フォームがメールアドレスの登録の有無を調べる手段になるため。
// 同じ理由で、照合するハッシュが無いときもbcryptの計算を行い、応答の時間を揃える。
func (v *SignInCreateValidator) Validate(ctx context.Context, input SignInCreateValidatorInput) (*model.User, error) {
	ve := model.NewValidationError()

	validateEmail(ctx, ve, input.Email)
	v.validatePassword(ctx, ve, input.Password)
	if ve.HasErrors() {
		return nil, ve
	}

	// 退会したユーザーは FindByEmail が返さないため、未知のメールアドレスと同じ扱いになる。
	user, err := v.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		auth.CheckPasswordWithoutDigest(input.Password)
		return nil, v.credentialsInvalid(ctx)
	}

	password, err := v.userPasswordRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if password == nil {
		auth.CheckPasswordWithoutDigest(input.Password)
		return nil, v.credentialsInvalid(ctx)
	}

	if err := auth.CheckPassword(password.PasswordDigest, input.Password); err != nil {
		return nil, v.credentialsInvalid(ctx)
	}

	return user, nil
}

// validatePassword はパスワードが未入力のときにエラーを記録する。
//
// 長さのポリシーはログインでは確かめない。受け入れるかどうかは保存済みのハッシュとの一致だけで決まり、
// ここでポリシーを確かめると、ポリシーを変える前に設定したパスワードでログインできなくなるだけのため。
func (v *SignInCreateValidator) validatePassword(ctx context.Context, ve *model.ValidationError, password string) {
	if password == "" {
		ve.AddField("password", i18n.T(ctx, "validation_required"))
	}
}

// credentialsInvalid は資格情報が一致しなかったことを伝えるエラーを返す。
func (v *SignInCreateValidator) credentialsInvalid(ctx context.Context) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, "validation_credentials_invalid"))
	return ve
}
