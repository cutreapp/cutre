package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetPasswordResetTokenUsecase は、パスワードリセットのメールのリンクが運ぶ平文のトークンから、使えるトークンを引く。
type GetPasswordResetTokenUsecase struct {
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
}

// NewGetPasswordResetTokenUsecase は GetPasswordResetTokenUsecase を生成する。
func NewGetPasswordResetTokenUsecase(passwordResetTokenRepo *repository.PasswordResetTokenRepository) *GetPasswordResetTokenUsecase {
	return &GetPasswordResetTokenUsecase{passwordResetTokenRepo: passwordResetTokenRepo}
}

// GetPasswordResetTokenInput は GetPasswordResetTokenUsecase.Execute の入力。
type GetPasswordResetTokenInput struct {
	// Token はパスワードリセットのメールのリンクに載っていたトークン。
	Token string
}

// GetPasswordResetTokenOutput は GetPasswordResetTokenUsecase.Execute の結果。
type GetPasswordResetTokenOutput struct {
	PasswordResetToken *model.PasswordResetToken
}

// Execute はトークンのダイジェストで期限内のトークンを引く。
// 無い・期限切れ・使用済みを区別せず、AppErrCodeResourceNotFound を返す。
func (uc *GetPasswordResetTokenUsecase) Execute(ctx context.Context, input GetPasswordResetTokenInput) (*GetPasswordResetTokenOutput, error) {
	resetToken, err := uc.passwordResetTokenRepo.FindLiveByTokenDigest(ctx, auth.HashToken(input.Token))
	if err != nil {
		return nil, fmt.Errorf("パスワードリセットのトークンの取得に失敗: %w", err)
	}
	if resetToken == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	return &GetPasswordResetTokenOutput{PasswordResetToken: resetToken}, nil
}
