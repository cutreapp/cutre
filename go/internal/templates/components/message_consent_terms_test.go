package components_test

import (
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates/components"
)

// TestMessageConsentTerms は、メッセージの取り扱いの文面を、渡したidの箇条書きで描画することを検証する。
func TestMessageConsentTerms(t *testing.T) {
	t.Parallel()

	got := render(t, components.MessageConsentTerms("message-consent-terms"))

	for _, want := range []string{
		`<ul id="message-consent-terms"`,
		"<li>メッセージは、あなたと交換の相手だけが見られます</li>",
		"<li>利用者から問題の報告があったときは、運営がその交換のメッセージ (取り消したものを含む) を確認することがあります</li>",
		"<li>取り消したメッセージの内容も、運営が保管します</li>",
		"<li>退会しても、交換の記録とメッセージは相手の画面に残ります</li>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
		}
	}
}
