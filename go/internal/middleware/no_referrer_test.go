package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestNoReferrer は、ページの描画とリダイレクトのどちらの応答にも参照元を送らせない方針が1つだけ付き、
// 先に middleware.SecurityHeaders が入れた既定値を上書きすることを検証する。
func TestNoReferrer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "ページの描画",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name: "リダイレクト",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/sign_up", http.StatusSeeOther)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler := middleware.SecurityHeaders(middleware.NoReferrer(tt.handler))
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/i/token", nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if got := resp.Header.Values("Referrer-Policy"); len(got) != 1 || got[0] != "no-referrer" {
				t.Errorf("Referrer-Policy = %v、期待値 = [no-referrer]", got)
			}
		})
	}
}
