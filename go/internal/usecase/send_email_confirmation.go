package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
)

// EmailConfirmationSender は確認コードのメールを送る。
// UseCaseがテンプレートに依存しないよう、メールの組み立ては実装 (email.ConfirmationSender) に任せる。
type EmailConfirmationSender interface {
	Send(ctx context.Context, to, code string, locale model.Locale, idempotencyKey string) error
}

// SendEmailConfirmationUsecase は確認コードのメールを送る。ジョブのワーカーから呼ばれる。
type SendEmailConfirmationUsecase struct {
	sender EmailConfirmationSender
}

// NewSendEmailConfirmationUsecase は SendEmailConfirmationUsecase を生成する。
func NewSendEmailConfirmationUsecase(sender EmailConfirmationSender) *SendEmailConfirmationUsecase {
	return &SendEmailConfirmationUsecase{sender: sender}
}

// SendEmailConfirmationInput は SendEmailConfirmationUsecase.Execute の入力。
type SendEmailConfirmationInput struct {
	Email  string
	Code   string
	Locale model.Locale
	// IdempotencyKey は再試行で同じメールを二重に送らないためのキー。ワーカーがジョブごとに決める。
	IdempotencyKey string
}

// Execute は確認コードのメールを送る。失敗はそのまま返し、ジョブの再試行に任せる。
func (uc *SendEmailConfirmationUsecase) Execute(ctx context.Context, input SendEmailConfirmationInput) error {
	if err := uc.sender.Send(ctx, input.Email, input.Code, input.Locale, input.IdempotencyKey); err != nil {
		return fmt.Errorf("確認コードのメールの送信に失敗: %w", err)
	}

	return nil
}
