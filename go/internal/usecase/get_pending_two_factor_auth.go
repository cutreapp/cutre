package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetPendingTwoFactorAuthUsecase は、認証アプリへの登録の途中の秘密鍵を画面に出す形で引く。
//
// コードの誤りで登録の画面を描き直すときに使う。秘密鍵を作り直すと、認証アプリへ登録した秘密鍵が無駄になるため。
type GetPendingTwoFactorAuthUsecase struct {
	twoFactorKey          *auth.TwoFactorKey
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository
}

// NewGetPendingTwoFactorAuthUsecase は GetPendingTwoFactorAuthUsecase を生成する。
func NewGetPendingTwoFactorAuthUsecase(twoFactorKey *auth.TwoFactorKey, userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository) *GetPendingTwoFactorAuthUsecase {
	return &GetPendingTwoFactorAuthUsecase{twoFactorKey: twoFactorKey, userTwoFactorAuthRepo: userTwoFactorAuthRepo}
}

// GetPendingTwoFactorAuthInput は GetPendingTwoFactorAuthUsecase.Execute の入力。
type GetPendingTwoFactorAuthInput struct {
	User *model.User
}

// GetPendingTwoFactorAuthOutput は GetPendingTwoFactorAuthUsecase.Execute の結果。
type GetPendingTwoFactorAuthOutput struct {
	// Setup は登録の途中の秘密鍵。登録の途中の設定が無いとき (有効にしたときを含む) はnil。
	Setup *TwoFactorAuthSetup
}

// Execute は登録の途中の秘密鍵を復号して返す。
func (uc *GetPendingTwoFactorAuthUsecase) Execute(ctx context.Context, input GetPendingTwoFactorAuthInput) (*GetPendingTwoFactorAuthOutput, error) {
	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, input.User.ID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if twoFactorAuth == nil || twoFactorAuth.IsEnabled() {
		return &GetPendingTwoFactorAuthOutput{}, nil
	}

	secret, err := decryptTwoFactorSecret(uc.twoFactorKey, twoFactorAuth)
	if err != nil {
		return nil, err
	}
	setup, err := newTwoFactorAuthSetup(secret, input.User)
	if err != nil {
		return nil, err
	}

	return &GetPendingTwoFactorAuthOutput{Setup: setup}, nil
}
