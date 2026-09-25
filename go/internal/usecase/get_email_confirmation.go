package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetEmailConfirmationUsecase は、Cookieが運ぶIDから確認済みでない確認を引く。
// 確認コードを再送するときに、送り先のメールアドレスを得るのに使う。
type GetEmailConfirmationUsecase struct {
	emailConfirmationRepo *repository.EmailConfirmationRepository
}

// NewGetEmailConfirmationUsecase は GetEmailConfirmationUsecase を生成する。
func NewGetEmailConfirmationUsecase(emailConfirmationRepo *repository.EmailConfirmationRepository) *GetEmailConfirmationUsecase {
	return &GetEmailConfirmationUsecase{emailConfirmationRepo: emailConfirmationRepo}
}

// GetEmailConfirmationOutput は GetEmailConfirmationUsecase.Execute の結果。
type GetEmailConfirmationOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute はIDの確認を引く。無い・確認済みのときは AppErrCodeResourceNotFound を返す。
func (uc *GetEmailConfirmationUsecase) Execute(ctx context.Context, id model.EmailConfirmationID) (*GetEmailConfirmationOutput, error) {
	confirmation, err := uc.emailConfirmationRepo.FindUnconfirmedByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("メールアドレスの確認の取得に失敗: %w", err)
	}
	if confirmation == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	return &GetEmailConfirmationOutput{EmailConfirmation: confirmation}, nil
}
