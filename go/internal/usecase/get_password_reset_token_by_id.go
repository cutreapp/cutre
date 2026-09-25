package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetPasswordResetTokenByIDUsecase は、Cookieが運ぶIDから使えるトークンと、その持ち主のユーザーを引く。
// 新しいパスワードの設定の画面で、どのアカウントのパスワードを変えるかを示すのに使う。
type GetPasswordResetTokenByIDUsecase struct {
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	userRepo               *repository.UserRepository
}

// NewGetPasswordResetTokenByIDUsecase は GetPasswordResetTokenByIDUsecase を生成する。
func NewGetPasswordResetTokenByIDUsecase(
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	userRepo *repository.UserRepository,
) *GetPasswordResetTokenByIDUsecase {
	return &GetPasswordResetTokenByIDUsecase{
		passwordResetTokenRepo: passwordResetTokenRepo,
		userRepo:               userRepo,
	}
}

// GetPasswordResetTokenByIDOutput は GetPasswordResetTokenByIDUsecase.Execute の結果。
type GetPasswordResetTokenByIDOutput struct {
	PasswordResetToken *model.PasswordResetToken
	User               *model.User
}

// Execute はIDの期限内のトークンと、その持ち主を引く。
// トークンが使えないとき、または持ち主が退会しているときは AppErrCodeResourceNotFound を返す。
func (uc *GetPasswordResetTokenByIDUsecase) Execute(ctx context.Context, id model.PasswordResetTokenID) (*GetPasswordResetTokenByIDOutput, error) {
	resetToken, err := uc.passwordResetTokenRepo.FindLiveByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("パスワードリセットのトークンの取得に失敗: %w", err)
	}
	if resetToken == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	user, err := uc.userRepo.FindByID(ctx, resetToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("パスワードリセットの対象ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	return &GetPasswordResetTokenByIDOutput{PasswordResetToken: resetToken, User: user}, nil
}
