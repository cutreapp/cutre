package sign_in_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// testContinuationKey は継続トークンの署名に使うテスト用の鍵。
const testContinuationKey = "test-continuation-token-key-0123456789"

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のトランザクションの中で動く Handler を組み立てる。
// レート制限のカウンターも同じトランザクションで数え、テストの後に残さない。
func newHandler(t *testing.T, cfg *config.Config, turnstileVerifier *testutil.FakeTurnstileVerifier) (*sign_in.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	userSessionRepo := repository.NewUserSessionRepository(db).WithTx(tx)

	return sign_in.NewHandler(
		cfg,
		session.NewManager(userSessionRepo),
		session.NewContinuationManager(testContinuationKey),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx)),
		turnstileVerifier,
		usecase.NewCreateSignInUsecase(
			validator.NewSignInCreateValidator(
				repository.NewUserRepository(db).WithTx(tx),
				repository.NewUserPasswordRepository(db).WithTx(tx),
			),
			repository.NewUserTwoFactorAuthRepository(db).WithTx(tx),
		),
		usecase.NewCreateSessionUsecase(userSessionRepo),
	), tx
}

func testConfig() *config.Config {
	return &config.Config{Env: "dev", Domain: "cutre.example.com"}
}

// TestNew は、ログイン画面が表示中の言語版へ送信するフォームを描画し、
// 安全な戻り先だけをフォームへ載せることを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		locale       string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:   "日本語版",
			target: "/sign_in",
			locale: i18n.LangJa,
			wantContains: []string{
				`<html lang="ja">`,
				"<title>ログイン | Cutre</title>",
				`action="/sign_in"`,
				`data-disable-on-submit`,
				`data-clear-on-history-restore`,
				`autocomplete="username"`,
				`autocomplete="current-password"`,
				`href="/password_reset"`,
			},
			// ログイン画面は検索からの入口として、インデックスを断らない。
			wantAbsent: []string{`name="return_to"`, "cf-turnstile", `name="robots"`},
		},
		{
			name:         "英語版",
			target:       "/en/sign_in",
			locale:       i18n.LangEn,
			wantContains: []string{`<html lang="en">`, `action="/en/sign_in"`, `data-clear-on-history-restore`, "Sign in", `href="/en/password_reset"`},
		},
		{
			name:         "安全な戻り先はフォームへ載せる",
			target:       "/sign_in?return_to=%2Fhome%3Ftab%3D1",
			locale:       i18n.LangJa,
			wantContains: []string{`name="return_to" value="/home?tab=1"`},
		},
		{
			name:       "別のオリジンを指す戻り先は捨てる",
			target:     "/sign_in?return_to=%2F%2Fevil.example.com",
			locale:     i18n.LangJa,
			wantAbsent: []string{`name="return_to"`, "evil.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, _ := newHandler(t, testConfig(), &testutil.FakeTurnstileVerifier{Passed: true})

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			rec := httptest.NewRecorder()

			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(body, absent) {
					t.Errorf("レスポンスボディに %q が含まれている", absent)
				}
			}
		})
	}
}

// TestNew_Turnstile は、サイトキーを設定したときだけウィジェットを描画することを検証する。
func TestNew_Turnstile(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.TurnstileSiteKey = "1x00000000000000000000AA"
	handler, _ := newHandler(t, cfg, &testutil.FakeTurnstileVerifier{Passed: true})

	rec := httptest.NewRecorder()
	handler.New(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))

	if body := rec.Body.String(); !strings.Contains(body, `data-sitekey="1x00000000000000000000AA"`) {
		t.Error("レスポンスボディにTurnstileのウィジェットが含まれていない")
	}
}

// TestNew_TurnstilePreconnect は表示とエラー再描画の両方で、ウィジェットを使う場合だけ接続準備することを確かめる。
func TestNew_TurnstilePreconnect(t *testing.T) {
	t.Parallel()
	for _, enabled := range []bool{false, true} {
		name := "無効"
		if enabled {
			name = "有効"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg := testConfig()
			if enabled {
				cfg.TurnstileSiteKey = "1x00000000000000000000AA"
			}
			handler, _ := newHandler(t, cfg, &testutil.FakeTurnstileVerifier{Passed: true})
			assertHint := func(rec *httptest.ResponseRecorder, status int) {
				t.Helper()
				if rec.Code != status {
					t.Fatalf("ステータス = %d、期待値 = %d", rec.Code, status)
				}
				html := rec.Body.String()
				hint := `<link rel="preconnect" href="https://challenges.cloudflare.com">`
				wantCount := 0
				if enabled {
					wantCount = 1
				}
				if count := strings.Count(html, hint); count != wantCount {
					t.Errorf("preconnectの数 = %d、期待値 = %d", count, wantCount)
				}
				if enabled && strings.Index(html, hint) > strings.Index(html, "</head>") {
					t.Error("preconnectがheadの外にある")
				}
			}
			rec := httptest.NewRecorder()
			handler.New(rec, httptest.NewRequest(http.MethodGet, "/sign_in", nil))
			assertHint(rec, http.StatusOK)
			email := testutil.UniqueEmail("preconnect")
			for i := range 11 {
				rec = httptest.NewRecorder()
				handler.Create(rec, postSignIn(url.Values{"email": {email}, "password": {"incorrect-password"}}))
				status := http.StatusUnprocessableEntity
				if i == 10 {
					status = http.StatusTooManyRequests
				}
				assertHint(rec, status)
			}
		})
	}
}
