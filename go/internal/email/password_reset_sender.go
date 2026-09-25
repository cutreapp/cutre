package email

import (
	"context"
	"net/url"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/emails/password_reset"
)

// PasswordResetSender はパスワードリセットのリンクを載せたメールを組み立てて送る。
type PasswordResetSender struct {
	sender Sender
	appURL string
}

// NewPasswordResetSender は PasswordResetSender を生成する。
// appURLはメールに載せるリンクの絶対URLを組み立てるのに使う (config.Config.AppURL)。
func NewPasswordResetSender(sender Sender, appURL string) *PasswordResetSender {
	return &PasswordResetSender{sender: sender, appURL: appURL}
}

// Send はトークンを載せたリンクのメールを、受け取る人の言語で送る。
// リンクもその言語版の、新しいパスワードを設定する画面を指す。
func (s *PasswordResetSender) Send(ctx context.Context, to, token string, locale model.Locale) error {
	ctx = i18n.SetLocale(ctx, string(locale))
	resetURL := s.appURL + i18n.LocalePath(string(locale), templates.PasswordPath) + "?" + url.Values{"token": {token}}.Encode()
	data := password_reset.Data{ResetURL: resetURL}

	return s.sender.Send(ctx, SendInput{
		To:       to,
		Subject:  i18n.T(ctx, "password_reset_email_subject"),
		HTMLBody: password_reset.HTML(data),
		TextBody: password_reset.Text(data),
	})
}
