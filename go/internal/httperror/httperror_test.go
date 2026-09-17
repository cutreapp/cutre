package httperror_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/i18n"
)

// testConfig は描画に必要な最小限の設定を返す。
// Envをdevにするのは、アセットバージョンをgitコマンドの有無に左右されない値にするため。
func testConfig() *config.Config {
	return &config.Config{Env: "dev", Domain: "cutre.example.com"}
}

// TestNotFound_Response は404の応答が、ステータス・Content-Type・キャッシュ方針のいずれも
// 明示された形で返ることを検証する。
func TestNotFound_Response(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	rec := httptest.NewRecorder()

	httperror.NewRenderer(testConfig()).NotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}

	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
	}

	// 後からこのアドレスにページを追加したとき、保存された404が覆い隠さないようにする。
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
}

// TestNotFound_RenderError は描画に失敗しても、HTMLを混ぜずに平文の404で応答することを検証する。
func TestNotFound_RenderError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	// templは描画開始時にcontextのエラーを確認するため、キャンセルによって描画失敗を再現する。
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	httperror.NewRenderer(testConfig()).NotFound(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	if got := rec.Body.String(); got != "Not Found\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Not Found\n")
	}
}

// TestNotFound_Locale は文言とトップページへの導線が、表示中の言語版に合わせて出ることを検証する。
func TestNotFound_Locale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		locale       string
		wantHeading  string
		wantBackHref string
	}{
		{
			name:         "日本語版",
			locale:       i18n.LangJa,
			wantHeading:  "ページが見つかりません",
			wantBackHref: `href="/"`,
		},
		{
			name:         "英語版",
			locale:       i18n.LangEn,
			wantHeading:  "Page not found",
			wantBackHref: `href="/en"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			httperror.NewRenderer(testConfig()).NotFound(rec, req)

			body := rec.Body.String()

			if !strings.Contains(body, `<html lang="`+tt.locale+`">`) {
				t.Errorf("出力に lang=%qのhtml要素が含まれていない", tt.locale)
			}
			if !strings.Contains(body, tt.wantHeading) {
				t.Errorf("出力に %qが含まれていない", tt.wantHeading)
			}
			// 行き止まりにしないための導線。表示中の言語版のトップページを指す。
			if !strings.Contains(body, tt.wantBackHref) {
				t.Errorf("出力に %qが含まれていない", tt.wantBackHref)
			}
		})
	}
}

// TestNotFound_NoCanonical は、存在しないアドレスを正規のアドレスとして名乗らず、
// 別の言語版として存在しないアドレスを指しもしないことを検証する。
func TestNotFound_NoCanonical(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	rec := httptest.NewRecorder()

	httperror.NewRenderer(testConfig()).NotFound(rec, req)

	body := rec.Body.String()

	for _, unwanted := range []string{`rel="canonical"`, `rel="alternate"`, `property="og:url"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("出力に %qが含まれている", unwanted)
		}
	}
}

// TestMethodNotAllowed_Response は405の応答が、ステータス・Content-Type・キャッシュ方針のいずれも
// 明示された形で返ることを検証する。
func TestMethodNotAllowed_Response(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	httperror.NewRenderer(testConfig()).MethodNotAllowed(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/html; charset=utf-8")
	}

	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	// Allowはどのメソッドを受け付けるかを知っているルーター側が付ける (cmd/cutre/serve.go)。
	if got := rec.Header().Values("Allow"); len(got) != 0 {
		t.Errorf("Allow = %v、期待値 = 空", got)
	}
}

// TestMethodNotAllowed_RenderError は描画に失敗しても、HTMLを混ぜずに平文の405で応答することを検証する。
// 404のフォールバックと共通の経路だが、応答するステータスがページごとに違うため個別に固定する。
func TestMethodNotAllowed_RenderError(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	// templは描画開始時にcontextのエラーを確認するため、キャンセルによって描画失敗を再現する。
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	httperror.NewRenderer(testConfig()).MethodNotAllowed(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "text/plain; charset=utf-8")
	}
	if got := rec.Body.String(); got != "Method Not Allowed\n" {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, "Method Not Allowed\n")
	}
}

// TestMethodNotAllowed_Locale は文言とトップページへの導線が、表示中の言語版に合わせて出ることを検証する。
func TestMethodNotAllowed_Locale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		locale       string
		wantHeading  string
		wantBackHref string
	}{
		{
			name:         "日本語版",
			locale:       i18n.LangJa,
			wantHeading:  "この操作は利用できません",
			wantBackHref: `href="/"`,
		},
		{
			name:         "英語版",
			locale:       i18n.LangEn,
			wantHeading:  "This action is not available",
			wantBackHref: `href="/en"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			httperror.NewRenderer(testConfig()).MethodNotAllowed(rec, req)

			body := rec.Body.String()

			if !strings.Contains(body, `<html lang="`+tt.locale+`">`) {
				t.Errorf("出力に lang=%qのhtml要素が含まれていない", tt.locale)
			}
			if !strings.Contains(body, tt.wantHeading) {
				t.Errorf("出力に %qが含まれていない", tt.wantHeading)
			}
			// 行き止まりにしないための導線。表示中の言語版のトップページを指す。
			if !strings.Contains(body, tt.wantBackHref) {
				t.Errorf("出力に %qが含まれていない", tt.wantBackHref)
			}
		})
	}
}
