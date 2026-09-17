package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

func TestRedirectSlashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		method       string
		target       string
		wantLocation string
	}{
		{name: "トップページは転送しない", target: "/"},
		{name: "正規化済みのパスは転送しない", target: "/en"},
		{name: "クエリの末尾スラッシュは対象外", target: "/en?next=/"},
		{name: "末尾スラッシュを除く", target: "/en/", wantLocation: "/en"},
		{name: "複数の末尾スラッシュをまとめて除く", target: "/en///", wantLocation: "/en"},
		{name: "スラッシュだけのパスはルートに着地する", target: "///", wantLocation: "/"},
		{name: "ハッシュをパスに保つ", target: "/a%23b/", wantLocation: "/a%23b"},
		{name: "疑問符をパスに保つ", target: "/a%3Fb/", wantLocation: "/a%3Fb"},
		{name: "パーセントをパスに保つ", target: "/a%25b/", wantLocation: "/a%25b"},
		{name: "途中のバックスラッシュをパスに保つ", target: "/a%5Cb/", wantLocation: "/a%5Cb"},
		{name: "末尾のバックスラッシュをパスに保つ", target: "/a/b%5C/", wantLocation: "/a/b%5C"},
		{name: "クエリの順序とエスケープを保つ", target: "/a%3Fb/?v=1&tag=a%26b&tag=c+d", wantLocation: "/a%3Fb?v=1&tag=a%26b&tag=c+d"},
		{name: "空のクエリを保つ", target: "/en/?", wantLocation: "/en?"},
		{name: "先頭の複数スラッシュで外部へ転送しない", target: "//example.com/", wantLocation: "/example.com"},
		{name: "バックスラッシュで外部へ転送しない", target: "/%5Cexample.com/", wantLocation: "/example.com"},
		{name: "小文字のエスケープでも外部へ転送しない", target: "/%5cexample.com/", wantLocation: "/example.com"},
		{name: "エスケープされた先頭スラッシュでも外部へ転送しない", target: "/%2Fexample.com/", wantLocation: "/example.com"},
		{name: "HEADはGETと同じ301で転送する", method: http.MethodHead, target: "/en/", wantLocation: "/en"},
		{name: "POSTはメソッドを保つ308で転送する", method: http.MethodPost, target: "/en/", wantLocation: "/en"},
		{name: "DELETEもメソッドを保つ308で転送する", method: http.MethodDelete, target: "/a/b/", wantLocation: "/a/b"},
		{name: "POSTでも正規化済みのパスは転送しない", method: http.MethodPost, target: "/en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			method := tt.method
			if method == "" {
				method = http.MethodGet
			}

			called := false
			handler := middleware.RedirectSlashes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.RequestURI != tt.target {
					t.Errorf("後続のRequestURI = %q、期待値 = %q", r.RequestURI, tt.target)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(method, tt.target, nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			// 301はメソッドをGETへ書き換えるクライアントがあるため、本文を持ちうるメソッドには308を使う。
			wantStatus := http.StatusMovedPermanently
			switch {
			case tt.wantLocation == "":
				wantStatus = http.StatusNoContent
			case method != http.MethodGet && method != http.MethodHead:
				wantStatus = http.StatusPermanentRedirect
			}
			if resp.StatusCode != wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", resp.StatusCode, wantStatus)
			}
			if got := resp.Header.Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if wantCalled := tt.wantLocation == ""; called != wantCalled {
				t.Errorf("後続の呼び出し = %t、期待値 = %t", called, wantCalled)
			}
		})
	}
}
