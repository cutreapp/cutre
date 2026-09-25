package home_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/home"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TestShow は、ホームがログイン中のユーザーを名指しし、言語版を持たないページとして
// canonicalも別言語版への参照も宣言せず、インデックスも断ることを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	handler := home.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"})

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangEn)
	ctx = middleware.SetUserToContext(ctx, &model.User{Atname: "cutre_user", Locale: model.LocaleEn})
	rec := httptest.NewRecorder()

	handler.Show(rec, req.WithContext(ctx))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`<html lang="en">`, "<title>Home | Cutre</title>", "Welcome, @cutre_user", `href="/@cutre_user"`, "Profile", `<meta name="robots" content="noindex">`} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	for _, absent := range []string{`rel="canonical"`, `rel="alternate"`, `rel="preconnect"`} {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かのホームとして描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	handler := home.NewHandler(&config.Config{Env: "dev", Domain: "cutre.example.com"})
	rec := httptest.NewRecorder()

	handler.Show(rec, httptest.NewRequest(http.MethodGet, "/home", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
