package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// wantSecurityHeaders は実装のsecurityHeadersを読み出さずに書き下している。
// 実装を言い換えるのではなく、レスポンスがどうあるべきかをテスト側で述べるため。
var wantSecurityHeaders = map[string]string{
	"Referrer-Policy":         "strict-origin-when-cross-origin",
	"X-Content-Type-Options":  "nosniff",
	"Content-Security-Policy": "frame-ancestors 'none'",
	"X-Frame-Options":         "DENY",
	"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	resp := rec.Result()
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("レスポンスボディのクローズのエラー = %v", err)
		}
	})

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	for name, want := range wantSecurityHeaders {
		// 値の個数も見るのは、二重になったヘッダー ("nosniff, nosniff") がHeader.Getでは
		// 付いているように見えるものの、読み手が受け取る値としては異なるため。
		values := resp.Header.Values(name)
		if len(values) != 1 {
			t.Errorf("%sの値の個数 = %d (%v)、期待値 = 1", name, len(values), values)
			continue
		}
		if values[0] != want {
			t.Errorf("%s = %q、期待値 = %q", name, values[0], want)
		}
	}
}

// TestSecurityHeaders_HandlerCanOverride は、後続のハンドラーが同じヘッダーを書き換えられることを固定する。
//
// ヘッダーを先に書き込む構造の裏返しで、埋め込みを許すページを将来足すときは
// ハンドラー側でframe-ancestorsを上書きすることになる。
func TestSecurityHeaders_HandlerCanOverride(t *testing.T) {
	t.Parallel()

	const want = "frame-ancestors 'self'"

	handler := middleware.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", want)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	resp := rec.Result()
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("レスポンスボディのクローズのエラー = %v", err)
		}
	})

	if got := resp.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != want {
		t.Errorf("Content-Security-Policy = %v、期待値 = [%q]", got, want)
	}
}
