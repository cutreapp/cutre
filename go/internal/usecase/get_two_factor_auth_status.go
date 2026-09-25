package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTwoFactorAuthStatusUsecase は、ユーザーが二要素認証を有効にしているかと、残りのリカバリーコードの数を引く。
type GetTwoFactorAuthStatusUsecase struct {
	userTwoFactorAuthRepo         *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository
}

// NewGetTwoFactorAuthStatusUsecase は GetTwoFactorAuthStatusUsecase を生成する。
func NewGetTwoFactorAuthStatusUsecase(
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
) *GetTwoFactorAuthStatusUsecase {
	return &GetTwoFactorAuthStatusUsecase{
		userTwoFactorAuthRepo:         userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo: userTwoFactorRecoveryCodeRepo,
	}
}

// GetTwoFactorAuthStatusInput は GetTwoFactorAuthStatusUsecase.Execute の入力。
type GetTwoFactorAuthStatusInput struct {
	UserID model.UserID
}

// GetTwoFactorAuthStatusOutput は GetTwoFactorAuthStatusUsecase.Execute の結果。
type GetTwoFactorAuthStatusOutput struct {
	// TwoFactorAuth は有効にしている二要素認証の設定。無効のとき (登録の途中を含む) はnil。
	TwoFactorAuth *model.UserTwoFactorAuth
	// UnusedRecoveryCodeCount はまだ使っていないリカバリーコードの数。無効のときは0。
	UnusedRecoveryCodeCount int
}

// Execute は二要素認証の状態を返す。
// 登録の途中の設定は、ログインでコードを求めないため無効として扱う。
func (uc *GetTwoFactorAuthStatusUsecase) Execute(ctx context.Context, input GetTwoFactorAuthStatusInput) (*GetTwoFactorAuthStatusOutput, error) {
	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if twoFactorAuth == nil || !twoFactorAuth.IsEnabled() {
		return &GetTwoFactorAuthStatusOutput{}, nil
	}

	count, err := uc.userTwoFactorRecoveryCodeRepo.CountUnused(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("リカバリーコードの数の取得に失敗: %w", err)
	}

	return &GetTwoFactorAuthStatusOutput{TwoFactorAuth: twoFactorAuth, UnusedRecoveryCodeCount: count}, nil
}
