package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TestUserLocale は、ログイン中のユーザーがいればURLから決めたロケールを users.locale で上書きし、
// 別の言語版の案内を消すことと、いなければそのまま通すことを検証する。
func TestUserLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		user          *model.User
		wantLocale    string
		wantSuggested string
	}{
		{name: "ログイン中は users.locale で表示する", user: &model.User{Locale: model.LocaleEn}, wantLocale: i18n.LangEn, wantSuggested: ""},
		{name: "未ログインならURLのロケールのまま", user: nil, wantLocale: i18n.LangJa, wantSuggested: i18n.LangEn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotLocale, gotSuggested string
			handler := middleware.UserLocale(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
				gotSuggested = i18n.GetSuggestedLocale(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/home", nil)
			ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
			ctx = i18n.SetSuggestedLocale(ctx, i18n.LangEn)
			if tt.user != nil {
				ctx = middleware.SetUserToContext(ctx, tt.user)
			}

			handler.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))

			if gotLocale != tt.wantLocale {
				t.Errorf("ロケール = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
			if gotSuggested != tt.wantSuggested {
				t.Errorf("案内するロケール = %q、期待値 = %q", gotSuggested, tt.wantSuggested)
			}
		})
	}
}
