package password_reset_sent_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset_sent"
	"github.com/cutreapp/cutre/go/internal/i18n"
)

// TestShow は、受け付けた後の画面を、送ったと断定しない案内とインデックスさせない指定付きで、表示中の言語版で描画することを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := password_reset_sent.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"})

	tests := []struct {
		name         string
		target       string
		locale       string
		wantContains []string
	}{
		{name: "日本語版", target: "/password_reset/sent", locale: i18n.LangJa, wantContains: []string{`name="robots"`, "登録されていれば", `href="/sign_in"`}},
		{name: "英語版", target: "/en/password_reset/sent", locale: i18n.LangEn, wantContains: []string{`<html lang="en">`, "If the email address you entered is registered", `href="/en/sign_in"`}},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.target, nil)
		req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
		rec := httptest.NewRecorder()
		handler.Show(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		for _, want := range tt.wantContains {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, want)
			}
		}
	}
}
