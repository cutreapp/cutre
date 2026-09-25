package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetConfirmedEmailConfirmationUsecase は、Cookieが運ぶIDから確認を済ませた確認を引く。
// アカウントの作成の画面で、登録するメールアドレスを示すのに使う。
type GetConfirmedEmailConfirmationUsecase struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewGetConfirmedEmailConfirmationUsecase は GetConfirmedEmailConfirmationUsecase を生成する。
func NewGetConfirmedEmailConfirmationUsecase(emailConfirmationRepo *repository.EmailConfirmationRepository) *GetConfirmedEmailConfirmationUsecase {
	return &GetConfirmedEmailConfirmationUsecase{emailConfirmationRepo: emailConfirmationRepo}
}

// GetConfirmedEmailConfirmationOutput は GetConfirmedEmailConfirmationUsecase.Execute の結果。
type GetConfirmedEmailConfirmationOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute はIDの確認を引く。無い・未確認のときは AppErrCodeResourceNotFound を返す。
func (uc *GetConfirmedEmailConfirmationUsecase) Execute(ctx context.Context, id model.EmailConfirmationID) (*GetConfirmedEmailConfirmationOutput, error) {
	confirmation, err := uc.emailConfirmationRepo.FindConfirmedByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("確認済みのメールアドレスの確認の取得に失敗: %w", err)
	}
	if confirmation == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	return &GetConfirmedEmailConfirmationOutput{EmailConfirmation: confirmation}, nil
}
