package email_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/cutreapp/cutre/go/internal/email"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// renderBody は本文を描画した文字列を返す。
// Senderが組み立てたメールは描画を送信の手段に任せるため、検証ではここで描画する。
func renderBody(t *testing.T, ctx context.Context, body templ.Component) string {
	t.Helper()

	var buf bytes.Buffer
	if err := body.Render(ctx, &buf); err != nil {
		t.Fatalf("本文の描画のエラー = %v", err)
	}

	return buf.String()
}

// TestConfirmationSender_Send は、確認コードを受け取る人の言語の件名・本文に載せて送ることを検証する。
func TestConfirmationSender_Send(t *testing.T) {
	t.Parallel()

	tests := []struct {
		locale      model.Locale
		wantSubject string
		wantLang    string
	}{
		{locale: model.LocaleJa, wantSubject: "Cutreの確認コード", wantLang: `<html lang="ja">`},
		{locale: model.LocaleEn, wantSubject: "Your Cutre confirmation code", wantLang: `<html lang="en">`},
	}

	for _, tt := range tests {
		fake := &testutil.FakeEmailSender{}
		ctx := context.Background()

		if err := email.NewConfirmationSender(fake).Send(ctx, "user@example.com", "012345", tt.locale, "job/1"); err != nil {
			t.Fatalf("%s: Send()のエラー = %v", tt.locale, err)
		}
		if len(fake.Sent) != 1 {
			t.Fatalf("%s: 送ったメールの件数 = %d、期待値 = 1", tt.locale, len(fake.Sent))
		}

		sent := fake.Sent[0]
		if sent.IdempotencyKey != "job/1" {
			t.Errorf("%s: IdempotencyKey = %q、期待値 = %q", tt.locale, sent.IdempotencyKey, "job/1")
		}
		if sent.To != "user@example.com" || sent.Subject != tt.wantSubject {
			t.Errorf("%s: 宛先・件名 = (%q, %q)、期待値 = (%q, %q)", tt.locale, sent.To, sent.Subject, "user@example.com", tt.wantSubject)
		}
		// Senderはctxに受け取る人のロケールを載せて渡すため、本文の描画もそのctxで行う。
		sentCtx := i18n.SetLocale(context.Background(), string(tt.locale))
		html := renderBody(t, sentCtx, sent.HTMLBody)
		if !strings.Contains(html, tt.wantLang) || !strings.Contains(html, "012345") {
			t.Errorf("%s: HTMLの本文 = %q、言語 %s と確認コードを含むことを期待", tt.locale, html, tt.wantLang)
		}
		if text := renderBody(t, sentCtx, sent.TextBody); !strings.Contains(text, "\n\n012345\n\n") {
			t.Errorf("%s: テキストの本文 = %q、確認コードを独立した段落に持つことを期待", tt.locale, text)
		}
	}
}

// TestAlreadyRegisteredNoticeSender_Send は、受け取る人の言語版のログイン画面のURLを載せて送ることを検証する。
func TestAlreadyRegisteredNoticeSender_Send(t *testing.T) {
	t.Parallel()

	tests := []struct {
		locale        model.Locale
		wantSubject   string
		wantSignInURL string
	}{
		{locale: model.LocaleJa, wantSubject: "Cutreのアカウントは登録済みです", wantSignInURL: "https://cutre.example.com/sign_in"},
		{locale: model.LocaleEn, wantSubject: "You already have a Cutre account", wantSignInURL: "https://cutre.example.com/en/sign_in"},
	}

	for _, tt := range tests {
		fake := &testutil.FakeEmailSender{}

		sender := email.NewAlreadyRegisteredNoticeSender(fake, "https://cutre.example.com")
		if err := sender.Send(context.Background(), "user@example.com", tt.locale, "job/2"); err != nil {
			t.Fatalf("%s: Send()のエラー = %v", tt.locale, err)
		}
		if len(fake.Sent) != 1 {
			t.Fatalf("%s: 送ったメールの件数 = %d、期待値 = 1", tt.locale, len(fake.Sent))
		}

		sent := fake.Sent[0]
		if sent.IdempotencyKey != "job/2" {
			t.Errorf("%s: IdempotencyKey = %q、期待値 = %q", tt.locale, sent.IdempotencyKey, "job/2")
		}
		if sent.Subject != tt.wantSubject {
			t.Errorf("%s: 件名 = %q、期待値 = %q", tt.locale, sent.Subject, tt.wantSubject)
		}
		sentCtx := i18n.SetLocale(context.Background(), string(tt.locale))
		for name, body := range map[string]templ.Component{"HTML": sent.HTMLBody, "テキスト": sent.TextBody} {
			if got := renderBody(t, sentCtx, body); !strings.Contains(got, tt.wantSignInURL) {
				t.Errorf("%s: %sの本文 = %q、%s を含むことを期待", tt.locale, name, got, tt.wantSignInURL)
			}
		}
	}
}

// TestPasswordResetSender_Send は、受け取る人の言語版の新しいパスワードを設定する画面へ、トークンを載せたリンクを送ることを検証する。
func TestPasswordResetSender_Send(t *testing.T) {
	t.Parallel()

	tests := []struct {
		locale       model.Locale
		wantSubject  string
		wantResetURL string
	}{
		{locale: model.LocaleJa, wantSubject: "Cutreのパスワードの再設定", wantResetURL: "https://cutre.example.com/password?token=abc_-123"},
		{locale: model.LocaleEn, wantSubject: "Reset your Cutre password", wantResetURL: "https://cutre.example.com/en/password?token=abc_-123"},
	}

	for _, tt := range tests {
		fake := &testutil.FakeEmailSender{}

		sender := email.NewPasswordResetSender(fake, "https://cutre.example.com")
		if err := sender.Send(context.Background(), "user@example.com", "abc_-123", tt.locale); err != nil {
			t.Fatalf("%s: Send()のエラー = %v", tt.locale, err)
		}
		if len(fake.Sent) != 1 {
			t.Fatalf("%s: 送ったメールの件数 = %d、期待値 = 1", tt.locale, len(fake.Sent))
		}

		sent := fake.Sent[0]
		if sent.To != "user@example.com" || sent.Subject != tt.wantSubject {
			t.Errorf("%s: 宛先・件名 = (%q, %q)、期待値 = (%q, %q)", tt.locale, sent.To, sent.Subject, "user@example.com", tt.wantSubject)
		}
		sentCtx := i18n.SetLocale(context.Background(), string(tt.locale))
		for name, body := range map[string]templ.Component{"HTML": sent.HTMLBody, "テキスト": sent.TextBody} {
			if got := renderBody(t, sentCtx, body); !strings.Contains(got, tt.wantResetURL) {
				t.Errorf("%s: %sの本文 = %q、%s を含むことを期待", tt.locale, name, got, tt.wantResetURL)
			}
		}
	}
}
