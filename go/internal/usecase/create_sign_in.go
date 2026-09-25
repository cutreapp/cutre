// Package usecase はアプリケーションの操作を1つずつ表すUseCaseを提供する。
// HandlerとWorkerはここを経由して、バリデーション・認可・永続化を行う。
package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateSignInUsecase はログインのフォームを受け付け、資格情報が一致したユーザーを返す。
//
// セッションの発行は CreateSessionUsecase の別の段階とする。
// 二要素認証を有効にしたユーザーは、資格情報の一致とセッションの発行の間に認証アプリのコードの入力が挟まるため。
// 資格情報の照合は読み取りだけで何も永続化しないため、トランザクションは開かない。
type CreateSignInUsecase struct {
	signInValidator       *validator.SignInCreateValidator
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewCreateSignInUsecase は CreateSignInUsecase を生成する。
func NewCreateSignInUsecase(
	signInValidator *validator.SignInCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
) *CreateSignInUsecase {
	return &CreateSignInUsecase{signInValidator: signInValidator, userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// CreateSignInInput は CreateSignInUsecase.Execute の入力。
type CreateSignInInput struct {
	Email    string
	Password string
}

// CreateSignInOutput は CreateSignInUsecase.Execute の結果。
type CreateSignInOutput struct {
	// User は資格情報が一致したユーザー。
	User *model.User
	// TwoFactorAuthRequired は、セッションを発行する前に認証アプリのコードを求めるかどうか。
	// 認証アプリへの登録の途中 (まだ有効にしていない) のユーザーには求めない。
	TwoFactorAuthRequired bool
}

// Execute は資格情報を照合し、一致したユーザーと、二要素認証を有効にしているかを返す。
// 入力の誤りと不一致は *model.ValidationError で返す。
func (uc *CreateSignInUsecase) Execute(ctx context.Context, input CreateSignInInput) (*CreateSignInOutput, error) {
	user, err := uc.signInValidator.Validate(ctx, validator.SignInCreateValidatorInput{
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		return nil, err
	}

	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}

	return &CreateSignInOutput{
		User:                  user,
		TwoFactorAuthRequired: twoFactorAuth != nil && twoFactorAuth.IsEnabled(),
	}, nil
}
