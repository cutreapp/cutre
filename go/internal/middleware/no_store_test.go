package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/middleware"
)

// TestNoStore は、ページの描画とリダイレクトのどちらの応答にも保存を禁じる方針が1つだけ付くことを検証する。
func TestNoStore(t *testing.T) {
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
				http.Redirect(w, r, "/home", http.StatusSeeOther)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			middleware.NoStore(tt.handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "private, no-store" {
				t.Errorf("Cache-Control = %v、期待値 = [private, no-store]", got)
			}
		})
	}
}

// TestNoStore_WithHTMLCache は、既定の方針を与える middleware.HTMLCache と重ねても、
// 保存を禁じる方針が既定値で緩まないことを検証する。
func TestNoStore_WithHTMLCache(t *testing.T) {
	t.Parallel()

	handler := middleware.HTMLCache(middleware.NoStore(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte("<!doctype html>")); err != nil {
			t.Errorf("レスポンスボディの書き込みのエラー = %v", err)
		}
	})))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))
	resp := rec.Result()
	t.Cleanup(func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("レスポンスボディのクローズのエラー = %v", err)
		}
	})

	if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != "private, no-store" {
		t.Errorf("Cache-Control = %v、期待値 = [private, no-store]", got)
	}
}
