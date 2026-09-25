package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// VerifyEmailConfirmationUsecase は入力された確認コードを照合し、メールアドレスの確認を済ませる。
//
// 誤ったコードでは誤入力の回数を増やす。検証の失敗でデータベースを更新するため、照合はValidatorではなくここで行う。
// ユーザーはまだ作らない。確認済みの確認を読むアカウントの作成が後の手順で行う。
type VerifyEmailConfirmationUsecase struct {
	emailConfirmationValidator *validator.EmailConfirmationCreateValidator
	emailConfirmationRepo      *repository.EmailConfirmationRepository
}

// NewVerifyEmailConfirmationUsecase は VerifyEmailConfirmationUsecase を生成する。
func NewVerifyEmailConfirmationUsecase(
	emailConfirmationValidator *validator.EmailConfirmationCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
) *VerifyEmailConfirmationUsecase {
	return &VerifyEmailConfirmationUsecase{
		emailConfirmationValidator: emailConfirmationValidator,
		emailConfirmationRepo:      emailConfirmationRepo,
	}
}

// VerifyEmailConfirmationInput は VerifyEmailConfirmationUsecase.Execute の入力。
type VerifyEmailConfirmationInput struct {
	// ID はCookieが運ぶ確認のID。
	ID   model.EmailConfirmationID
	Code string
}

// VerifyEmailConfirmationOutput は VerifyEmailConfirmationUsecase.Execute の結果。
type VerifyEmailConfirmationOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute はコードの形式を検証してから照合する。
//
// 誤ったコードはコードの欄のエラーとして、照合できない確認 (期限切れ・誤入力の回数が上限に達した・確認済み) は
// コードの再送を促すフォーム全体のエラーとして、どちらも *model.ValidationError で返す。
// 上限に達した誤入力は、その時点で照合できない確認と同じ扱いにし、残りの無い入力を求めない。
func (uc *VerifyEmailConfirmationUsecase) Execute(ctx context.Context, input VerifyEmailConfirmationInput) (*VerifyEmailConfirmationOutput, error) {
	if err := uc.emailConfirmationValidator.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{Code: input.Code}); err != nil {
		return nil, err
	}

	confirmation, err := uc.emailConfirmationRepo.Verify(ctx, input.ID, input.Code)
	if err != nil {
		return nil, fmt.Errorf("確認コードの照合に失敗: %w", err)
	}

	ve := model.NewValidationError()
	switch {
	case confirmation == nil || confirmation.HasReachedMaxFailedAttempts():
		ve.AddGlobal(i18n.T(ctx, "validation_confirmation_code_unusable"))
		return nil, ve
	case confirmation.ConfirmedAt == nil:
		ve.AddField("code", i18n.T(ctx, "validation_confirmation_code_incorrect"))
		return nil, ve
	}

	return &VerifyEmailConfirmationOutput{EmailConfirmation: confirmation}, nil
}
