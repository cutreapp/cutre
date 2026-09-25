// Package email はメールの描画と送信を扱う。
//
// 送信の手段は Sender の実装で切り替える。本番はResendのAPIで送り、APIキーを設定していない環境では送らずにログへ出力する。
// メールの種類ごとの送信 (件名と本文の組み立て) は、そのメールを使うタスクでこのパッケージに足す。
package email

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/resend/resend-go/v2"
)

// fromName は送信するメールのFromに添える表示名。
// サービス名は固有名詞でロケールによらないため、翻訳ファイルではなくここに置く。
const fromName = "Cutre"

// requestTimeout はResendのAPIへの1回の呼び出しの上限。
// 応答が返らないまま、ジョブを処理するワーカーを塞ぎ続けないようにする。
const requestTimeout = 30 * time.Second

// Sender は描画したメールを送る。
type Sender interface {
	Send(ctx context.Context, input SendInput) error
}

// SendInput は1通のメールの送信に必要な値。
type SendInput struct {
	// To は送信先のメールアドレス。
	To string
	// Subject は件名。
	Subject string
	// HTMLBody はHTMLの本文。
	HTMLBody templ.Component
	// TextBody はテキストの本文。HTMLを表示しないメールクライアント向けに必ず添える。
	TextBody templ.Component
	// IdempotencyKey は再試行時に同じメールの重複送信を防ぐためのキー。不要な場合は空にする。
	IdempotencyKey string
}

// ResendSender はResendのAPIでメールを送る。
type ResendSender struct {
	client     *resend.Client
	httpClient *http.Client
	from       string
}

// NewResendSender は ResendSender を生成する。
func NewResendSender(apiKey, fromEmail string) *ResendSender {
	httpClient := &http.Client{Timeout: requestTimeout}

	return &ResendSender{
		client:     resend.NewCustomClient(httpClient, apiKey),
		httpClient: httpClient,
		from:       fmt.Sprintf("%s <%s>", fromName, fromEmail),
	}
}

// Send は本文を描画し、ResendのAPIへ送る。
func (s *ResendSender) Send(ctx context.Context, input SendInput) error {
	html, text, err := render(ctx, input)
	if err != nil {
		return err
	}

	if _, err := s.client.Emails.SendWithOptions(ctx, &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{input.To},
		Subject: input.Subject,
		Html:    html,
		Text:    text,
	}, &resend.SendEmailOptions{IdempotencyKey: input.IdempotencyKey}); err != nil {
		return fmt.Errorf("メールの送信 (Resend) に失敗: %w", err)
	}

	return nil
}

// LogSender はメールを送らず、ログへ出力する。
// 開発環境で、確認コードなどメールで届く値を実際のメールボックス無しに読めるようにする。
type LogSender struct{}

// NewLogSender は LogSender を生成する。
func NewLogSender() *LogSender {
	return &LogSender{}
}

// Send は本文を描画し、テキストの本文をログへ出力する。
// HTMLの本文も描画するのは、テンプレートの誤りを送信の経路と同じ時点で気付けるようにするため。
func (s *LogSender) Send(ctx context.Context, input SendInput) error {
	_, text, err := render(ctx, input)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "メールを送らずにログへ出力しました", "to", input.To, "subject", input.Subject, "body", text)

	return nil
}

// render はHTMLとテキストの本文を文字列へ描画する。
func render(ctx context.Context, input SendInput) (html, text string, err error) {
	var htmlBuf, textBuf bytes.Buffer

	if err := input.HTMLBody.Render(ctx, &htmlBuf); err != nil {
		return "", "", fmt.Errorf("HTMLの本文の描画に失敗: %w", err)
	}
	if err := input.TextBody.Render(ctx, &textBuf); err != nil {
		return "", "", fmt.Errorf("テキストの本文の描画に失敗: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}
