package email

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates/emails/email_confirmation"
)

// ConfirmationSender は確認コードのメールを組み立てて送る。
type ConfirmationSender struct {
	sender Sender
}

// NewConfirmationSender は ConfirmationSender を生成する。
func NewConfirmationSender(sender Sender) *ConfirmationSender {
	return &ConfirmationSender{sender: sender}
}

// Send は確認コードのメールを、受け取る人の言語で送る。
// idempotencyKeyは、同じメールを再試行したときに二重に届かないようにするキー (SendInput.IdempotencyKey)。
func (s *ConfirmationSender) Send(ctx context.Context, to, code string, locale model.Locale, idempotencyKey string) error {
	ctx = i18n.SetLocale(ctx, string(locale))
	data := email_confirmation.Data{Code: code}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  i18n.T(ctx, "email_confirmation_email_subject"),
		HTMLBody: email_confirmation.HTML(data),
		TextBody: email_confirmation.Text(data),

		IdempotencyKey: idempotencyKey,
	})
}
