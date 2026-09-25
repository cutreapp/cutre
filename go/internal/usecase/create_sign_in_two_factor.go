package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateSignInTwoFactorUsecase は、パスワードを確かめたユーザーの認証アプリのコードを照合し、ログインを続けてよいユーザーを返す。
//
// セッションの発行は CreateSignInUsecase と同じく CreateSessionUsecase の別の段階とする。
// 書き込みは使ったタイムステップの記録 (条件付きのUPDATE) 1回のため、トランザクションは開かない。
type CreateSignInTwoFactorUsecase struct {
	twoFactorKey             *auth.TwoFactorKey
	signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator
	userRepo                 *repository.UserRepository
	userTwoFactorAuthRepo    *repository.UserTwoFactorAuthRepository
}

// NewCreateSignInTwoFactorUsecase は CreateSignInTwoFactorUsecase を生成する。
func NewCreateSignInTwoFactorUsecase(
	twoFactorKey *auth.TwoFactorKey,
	signInTwoFactorValidator *validator.SignInTwoFactorCreateValidator,
	userRepo *repository.UserRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *CreateSignInTwoFactorUsecase {
	return &CreateSignInTwoFactorUsecase{
		twoFactorKey:             twoFactorKey,
		signInTwoFactorValidator: signInTwoFactorValidator,
		userRepo:                 userRepo,
		userTwoFactorAuthRepo:    userTwoFactorAuthRepo,
	}
}

// CreateSignInTwoFactorInput は CreateSignInTwoFactorUsecase.Execute の入力。
type CreateSignInTwoFactorInput struct {
	// UserID はパスワードを確かめたユーザーのID。署名付きのCookieから読んだ値を渡す。
	UserID model.UserID
	// Code は入力されたままのコード。空白・ハイフン・全角の数字はここで正規化する。
	Code string
}

// CreateSignInTwoFactorOutput は CreateSignInTwoFactorUsecase.Execute の結果。
type CreateSignInTwoFactorOutput struct {
	// User はコードが合ったユーザー。
	User *model.User
}

// Execute はコードを照合し、合っていればそのタイムステップを使用済みにしてユーザーを返す。
//
// 形式の誤り・照合の失敗・使用済みのコードは、コードの欄の *model.ValidationError で返す。
// 使用済みのコードも照合の失敗と同じ文言にし、認証アプリの最新のコードを入れ直してもらう。
// ユーザーが退会した・二要素認証を無効にしたときは、パスワードの確認からやり直させるため AppErrCodeConflict を返す。
func (uc *CreateSignInTwoFactorUsecase) Execute(ctx context.Context, input CreateSignInTwoFactorInput) (*CreateSignInTwoFactorOutput, error) {
	code := auth.NormalizeTOTPCode(input.Code)
	if err := uc.signInTwoFactorValidator.Validate(ctx, validator.SignInTwoFactorCreateValidatorInput{Code: code}); err != nil {
		return nil, err
	}

	user, err := uc.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, &model.AppError{Code: model.AppErrCodeConflict}
	}

	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if twoFactorAuth == nil || !twoFactorAuth.IsEnabled() {
		return nil, &model.AppError{Code: model.AppErrCodeConflict}
	}

	secret, err := decryptTwoFactorSecret(uc.twoFactorKey, twoFactorAuth)
	if err != nil {
		return nil, err
	}
	step, ok := auth.MatchTOTPCode(secret, code, time.Now())
	if !ok {
		return nil, incorrectTOTPCodeError(ctx)
	}

	used, err := uc.userTwoFactorAuthRepo.UseStep(ctx, user.ID, step)
	if err != nil {
		return nil, fmt.Errorf("使ったタイムステップの記録に失敗: %w", err)
	}
	if !used {
		return nil, incorrectTOTPCodeError(ctx)
	}

	return &CreateSignInTwoFactorOutput{User: user}, nil
}

// incorrectTOTPCodeError はコードが合わなかったことを、コードの欄のエラーで返す。
func incorrectTOTPCodeError(ctx context.Context) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddField("code", i18n.T(ctx, "validation_totp_code_incorrect"))

	return ve
}
