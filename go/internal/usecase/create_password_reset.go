package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreatePasswordResetUsecase はパスワードリセットの申請を受け付け、リセットのメールを送るジョブを投入する。
//
// 未登録のアドレスでは何もせず、登録済みのときと同じ結果を返す。呼び出し側は同じ画面を出し、登録の有無を明かさない。
// リンクのトークンはここでは作らない。ジョブを処理するときに作り、平文のトークンをデータベースに残さないため。
type CreatePasswordResetUsecase struct {
	passwordResetValidator *validator.PasswordResetCreateValidator
	dispatcher             *dispatcher.Dispatcher
}

// NewCreatePasswordResetUsecase は CreatePasswordResetUsecase を生成する。
func NewCreatePasswordResetUsecase(
	passwordResetValidator *validator.PasswordResetCreateValidator,
	dispatcher *dispatcher.Dispatcher,
) *CreatePasswordResetUsecase {
	return &CreatePasswordResetUsecase{
		passwordResetValidator: passwordResetValidator,
		dispatcher:             dispatcher,
	}
}

// CreatePasswordResetInput は CreatePasswordResetUsecase.Execute の入力。
type CreatePasswordResetInput struct {
	Email string
	// Locale はメールを書く言語。申請の画面の言語版で決まる。
	Locale model.Locale
}

// Execute はメールアドレスを検証し、登録済みのユーザーのときだけリセットのメールを送るジョブを投入する。
func (uc *CreatePasswordResetUsecase) Execute(ctx context.Context, input CreatePasswordResetInput) error {
	user, err := uc.passwordResetValidator.Validate(ctx, validator.PasswordResetCreateValidatorInput{Email: input.Email})
	if err != nil {
		return err
	}
	if user == nil {
		return nil
	}

	if err := uc.dispatcher.EnqueuePasswordReset(ctx, user.ID, input.Locale); err != nil {
		return fmt.Errorf("パスワードリセットのメールを送るジョブの投入に失敗: %w", err)
	}

	return nil
}
