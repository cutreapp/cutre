package password_reset_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のデータベースに直接書き込む Handler を組み立てる。
func newHandler(t *testing.T, turnstileVerifier *testutil.FakeTurnstileVerifier) *password_reset.Handler {
	t.Helper()

	db := testutil.GetTestDB()
	jobs, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}

	return password_reset.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		turnstileVerifier,
		usecase.NewCreatePasswordResetUsecase(validator.NewPasswordResetCreateValidator(repository.NewUserRepository(db)), jobs),
	)
}

// newRequest はロケールを載せたリクエストを返す。remoteAddrはレート制限を数える単位を他のテストと分けるために渡す。
func newRequest(method, target string, form url.Values, locale string, remoteAddr string) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.RemoteAddr = remoteAddr

	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// assertBody は、応答の本文が wantContains をすべて含み、wantAbsent をどれも含まないことを確かめる。
func assertBody(t *testing.T, name, body string, wantContains, wantAbsent []string) {
	t.Helper()

	for _, want := range wantContains {
		if !strings.Contains(body, want) {
			t.Errorf("%s: レスポンスボディに %q が含まれていない", name, want)
		}
	}
	for _, absent := range wantAbsent {
		if strings.Contains(body, absent) {
			t.Errorf("%s: レスポンスボディに %q が含まれている", name, absent)
		}
	}
}

// TestNew は、表示中の言語版へ送るメールアドレスの入力フォームを、インデックスを許して描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})

	tests := []struct {
		name         string
		target       string
		locale       string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:   "日本語版",
			target: "/password_reset",
			locale: i18n.LangJa,
			wantContains: []string{
				`action="/password_reset"`,
				`autocomplete="username"`,
				`aria-describedby="email-hint"`,
				`data-disable-on-submit`,
				`<link rel="canonical"`,
				`href="/sign_in"`,
			},
			wantAbsent: []string{`name="robots"`},
		},
		{
			name:         "英語版",
			target:       "/en/password_reset",
			locale:       i18n.LangEn,
			wantContains: []string{`<html lang="en">`, `action="/en/password_reset"`, `href="/en/sign_in"`, "Send reset link"},
		},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		handler.New(rec, newRequest(http.MethodGet, tt.target, nil, tt.locale, "192.0.2.50:1234"))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusOK)
		}
		assertBody(t, tt.name, rec.Body.String(), tt.wantContains, tt.wantAbsent)
	}
}
