package email

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/emails/already_registered_notice"
)

// AlreadyRegisteredNoticeSender は、登録済みのメールアドレスで登録を始めた人へ、ログインを案内するメールを組み立てて送る。
type AlreadyRegisteredNoticeSender struct {
	sender Sender
	appURL string
}

// NewAlreadyRegisteredNoticeSender は AlreadyRegisteredNoticeSender を生成する。
// appURLはメールに載せるログイン画面の絶対URLを組み立てるのに使う (config.Config.AppURL)。
func NewAlreadyRegisteredNoticeSender(sender Sender, appURL string) *AlreadyRegisteredNoticeSender {
	return &AlreadyRegisteredNoticeSender{sender: sender, appURL: appURL}
}

// Send はログインを案内するメールを、受け取る人の言語で送る。
// リンクもその言語版のログイン画面を指す。
// idempotencyKeyは、同じメールを再試行したときに二重に届かないようにするキー (SendInput.IdempotencyKey)。
func (s *AlreadyRegisteredNoticeSender) Send(ctx context.Context, to string, locale model.Locale, idempotencyKey string) error {
	ctx = i18n.SetLocale(ctx, string(locale))
	data := already_registered_notice.Data{SignInURL: s.appURL + i18n.LocalePath(string(locale), templates.SignInPath)}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  i18n.T(ctx, "already_registered_notice_email_subject"),
		HTMLBody: already_registered_notice.HTML(data),
		TextBody: already_registered_notice.Text(data),

		IdempotencyKey: idempotencyKey,
	})
}
