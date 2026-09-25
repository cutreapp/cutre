package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// testConfig はルーターの組み立てに必要な最小限の設定を返す。
// Envをdevにするのは、アセットバージョンをgitコマンドの有無に左右されない値にするため。
func testConfig() *config.Config {
	return &config.Config{
		Env:                  "dev",
		Domain:               "cutre.example.com",
		ContinuationTokenKey: "test-continuation-token-key-0123456789",
		TOTPEncryptionKey:    "test-totp-encryption-key-0123456789",
	}
}

// withCSRFToken はCSRFの検証を通すためのトークンをリクエストに載せて返す。
// 安全でないメソッドはルーティングより先にCSRFの検証を受けるため、
// その先 (405の応答など) を確かめるテストはトークンを持って入る必要がある。
func withCSRFToken(req *http.Request) *http.Request {
	const token = "csrf-token"

	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
	req.Header.Set(middleware.CSRFHeaderName, token)

	return req
}

func TestNewRouter_Health(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q、期待値 = %q", got, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("レスポンスボディのデコードのエラー = %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q、期待値 = %q", body["status"], "ok")
	}
}

// TestNewRouter_EmailConfirmation は送信後の遷移先を両言語で開けることを検証する。
func TestNewRouter_EmailConfirmation(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	db := testutil.GetTestDB()
	router := newRouter(cfg, db, t.TempDir())
	mgr := session.NewContinuationManager(cfg.ContinuationTokenKey)
	invitationID := testutil.NewInvitationBuilder(t, db).Build()
	cookies := httptest.NewRecorder()
	mgr.SetInvitationID(cookies, invitationID)
	mgr.SetEmailConfirmationID(cookies, model.EmailConfirmationID(uuid.New()))

	for _, path := range []string{"/email_confirmation", "/en/email_confirmation"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, cookie := range cookies.Result().Cookies() {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータス = %d、期待値 = 200", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `autocomplete="one-time-code"`) {
			t.Errorf("%s: 確認コードの入力欄が無い", path)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("%s: Cache-Control = %q、期待値 = private, no-store", path, got)
		}
	}
}

// TestNewRouter_EmailConfirmationSubmit は、確認コードの照合 (POST) と、フォームから _method で送る再送 (PATCH) が
// 両言語のパスでハンドラーに届くことを固定する。
func TestNewRouter_EmailConfirmationSubmit(t *testing.T) {
	t.Parallel()

	const token = "csrf-token"
	cfg := testConfig()
	db := testutil.GetTestDB()
	router := newRouter(cfg, db, t.TempDir())
	mgr := session.NewContinuationManager(cfg.ContinuationTokenKey)

	tests := []struct {
		name         string
		form         url.Values
		wantStatus   int
		wantRedirect bool
	}{
		{name: "照合", form: url.Values{"code": {"000000"}}, wantStatus: http.StatusUnprocessableEntity},
		{name: "再送", form: url.Values{"_method": {http.MethodPatch}}, wantStatus: http.StatusSeeOther, wantRedirect: true},
	}
	for _, tt := range tests {
		for _, prefix := range []string{"", "/en"} {
			cookies := httptest.NewRecorder()
			mgr.SetInvitationID(cookies, testutil.NewInvitationBuilder(t, db).Build())
			mgr.SetEmailConfirmationID(cookies, testutil.NewEmailConfirmationBuilder(t, db).Build())

			form := url.Values{middleware.CSRFFieldName: {token}}
			for key, values := range tt.form {
				form[key] = values
			}
			path := prefix + "/email_confirmation"
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
			for _, cookie := range cookies.Result().Cookies() {
				req.AddCookie(cookie)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s: ステータス = %d、期待値 = %d", tt.name, path, rec.Code, tt.wantStatus)
			}
			// 再送は同じ言語版の入力画面へ戻す。
			if tt.wantRedirect && rec.Header().Get("Location") != path {
				t.Errorf("%s %s: Location = %q、期待値 = %q", tt.name, path, rec.Header().Get("Location"), path)
			}
		}
	}
}

// TestNewRouter_Account は、アカウントの作成の画面と送信が両言語のパスでハンドラーに届き、
// HTTPキャッシュに保存させないことを固定する。
func TestNewRouter_Account(t *testing.T) {
	t.Parallel()

	const token = "csrf-token"
	cfg := testConfig()
	db := testutil.GetTestDB()
	router := newRouter(cfg, db, t.TempDir())
	mgr := session.NewContinuationManager(cfg.ContinuationTokenKey)

	tests := []struct {
		method     string
		wantStatus int
	}{
		{method: http.MethodGet, wantStatus: http.StatusOK},
		// 空のアットネームとパスワードは受け付けず、フォームを再描画する。
		{method: http.MethodPost, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		for _, path := range []string{"/account", "/en/account"} {
			cookies := httptest.NewRecorder()
			mgr.SetInvitationID(cookies, testutil.NewInvitationBuilder(t, db).Build())
			mgr.SetConfirmedEmailConfirmationID(cookies, testutil.NewEmailConfirmationBuilder(t, db).WithConfirmedAt(time.Now()).Build())

			form := url.Values{middleware.CSRFFieldName: {token}}
			req := httptest.NewRequest(tt.method, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
			for _, cookie := range cookies.Result().Cookies() {
				req.AddCookie(cookie)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s: ステータス = %d、期待値 = %d", tt.method, path, rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
				t.Errorf("%s %s: Cache-Control = %q、期待値 = private, no-store", tt.method, path, got)
			}
		}
	}
}

// TestNewRouter_Welcome は各言語版のトップページがそれぞれのパスに登録され、その言語のHTMLを返すことを固定する。
// ハンドラー単体のテストとは別に置くのは、登録先のパスを取り違えても
// welcome.Show 自体のテストは成功し、トップページが404になったことを検出できないため。
func TestNewRouter_Welcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		// 描画の中身は internal/handler/welcome のテストが持つため、ここでは
		// ルーターが返したものがどの言語版のトップページかを見出しだけで確かめる。
		wantHeading string
	}{
		{
			name:        "日本語版",
			path:        "/",
			wantHeading: "Cutreにようこそ！",
		},
		{
			name:        "英語版",
			path:        "/en",
			wantHeading: "Welcome to Cutre!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestNewRouter_I18n は newRouter が middleware.I18n を登録していることを固定する。
// ミドルウェア自体の挙動は internal/middleware のテストが持つため、ここで見るのは配線だけ。
func TestNewRouter_I18n(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantLocale string
	}{
		{
			name:       "英語版のパスのロケールがルーター経由でcontextに載る",
			path:       "/en/test-locale",
			wantLocale: i18n.LangEn,
		},
		{
			name:       "言語コードを持たないパスではデフォルトロケールが載る",
			path:       "/test-locale",
			wantLocale: i18n.DefaultLang,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// newRouter が返すルーターに検証用のルートを足し、登録済みのミドルウェアを
			// 通したあとのcontextを観測する。ルーターはテストごとに作り直すため共有されない。
			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			var gotLocale string
			observe := func(_ http.ResponseWriter, r *http.Request) {
				gotLocale = i18n.GetLocale(r.Context())
			}
			router.Get("/test-locale", observe)
			router.Get("/en/test-locale", observe)

			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			if gotLocale != tt.wantLocale {
				t.Errorf("ロケール = %q、期待値 = %q", gotLocale, tt.wantLocale)
			}
		})
	}
}

// TestNewRouter_LocaleIndependentRoutes は言語版を持たないルートが、
// ロケールの判定に左右されずに同じ応答を返すことを固定する。
// これらは公開コンテンツではないため、言語版のURLを持たず、Accept-Languageでも内容が変わらない。
func TestNewRouter_LocaleIndependentRoutes(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	const css = "body{color:red}"
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte(css), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "ヘルスチェック",
			path: "/health",
			want: `{"status":"ok"}`,
		},
		{
			name: "静的アセット",
			path: "/static/css/style.css",
			want: css,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)

			for _, acceptLanguage := range []string{"", "en-US,en;q=0.9", "ja"} {
				req := httptest.NewRequest(http.MethodGet, tt.path, nil)
				if acceptLanguage != "" {
					req.Header.Set("Accept-Language", acceptLanguage)
				}
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("Accept-Language %qのステータスコード = %d、期待値 = %d", acceptLanguage, rec.Code, http.StatusOK)
				}
				if got := strings.TrimSpace(rec.Body.String()); got != tt.want {
					t.Errorf("Accept-Language %qのレスポンスボディ = %q、期待値 = %q", acceptLanguage, got, tt.want)
				}
			}
		})
	}
}

// TestNewRouter_StaticAssets は /static/* が配信元ディレクトリの中身を返すことを固定する。
// 配信元をテスト用のディレクトリにするのは、pnpm buildの生成物をGoのテストの前提にしないため。
func TestNewRouter_StaticAssets(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	const want = "body{color:red}"
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte(want), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)

	req := httptest.NewRequest(http.MethodGet, "/static/css/style.css", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Errorf("Content-Type = %q、期待値 = text/cssで始まる値", got)
	}

	if got := rec.Body.String(); got != want {
		t.Errorf("レスポンスボディ = %q、期待値 = %q", got, want)
	}
}

// TestNewRouter_StaticAssetsSkipSession は、静的アセットが利用者ごとに値の変わるミドルウェア
// (セッション・CSRF・フラッシュ) の対象から外れ、
// 利用者固有のSet-Cookieをpublicなキャッシュへ混ぜないことを固定する。
func TestNewRouter_StaticAssetsSkipSession(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body{}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	db := testutil.GetTestDB()
	userSession := testutil.NewUserSessionBuilder(t, db).
		WithUserID(testutil.NewUserBuilder(t, db).Build()).
		WithLastSeenAt(time.Now().Add(-48 * time.Hour)).
		WithExpiresAt(time.Now().Add(24 * time.Hour))
	userSession.Build()

	cfg := testConfig()
	cfg.Env = "prod"
	router := newRouter(cfg, db, staticDir)

	req := httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
	req.AddCookie(&http.Cookie{Name: session.FlashCookieName, Value: "flash"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Errorf("Set-Cookie = %q、期待値 = 空文字列", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, "public, max-age=31536000, immutable")
	}
}

// TestNewRouter_LocaleSuggestion は言語版の案内が、ミドルウェアの判定からページのHTMLまで
// 通しで届くことを固定する。
//
// 判定 (middleware) ・組み立て (viewmodel) ・描画 (templates) はそれぞれ単体のテストを持つが、
// どこか1つの配線が外れても各単体テストは成功する。
func TestNewRouter_LocaleSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		path           string
		acceptLanguage string
		selectedLocale string
		wantMessage    string
	}{
		{
			name:           "日本語版を英語話者が見ると英語で案内が出る",
			path:           "/",
			acceptLanguage: "en-US,en;q=0.9",
			wantMessage:    "This page is also available in English.",
		},
		{
			name:           "英語版を日本語話者が見ると日本語で案内が出る",
			path:           "/en",
			acceptLanguage: "ja",
			wantMessage:    "このページは日本語でも読めます。",
		},
		{
			name:           "求める言語版を見ているときは案内が出ない",
			path:           "/en",
			acceptLanguage: "en",
			wantMessage:    "",
		},
		{
			name:           "選んだ言語版を見ているときは出ない",
			path:           "/",
			acceptLanguage: "en",
			selectedLocale: i18n.LangJa,
			wantMessage:    "",
		},
		{
			name:           "選んだ言語と違う言語版を開くと選んだ言語で案内が出る",
			path:           "/en",
			acceptLanguage: "en",
			selectedLocale: i18n.LangJa,
			wantMessage:    "このページは日本語でも読めます。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Language", tt.acceptLanguage)
			if tt.selectedLocale != "" {
				req.AddCookie(&http.Cookie{Name: middleware.SelectedLocaleCookie, Value: tt.selectedLocale})
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			body := rec.Body.String()

			if got := strings.Contains(body, "data-locale-suggestion"); got != (tt.wantMessage != "") {
				t.Errorf("案内の出力有無 = %t、期待値 = %t", got, tt.wantMessage != "")
			}
			if tt.wantMessage != "" && !strings.Contains(body, tt.wantMessage) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantMessage)
			}
		})
	}
}

// TestNewRouter_SecurityHeaders は newRouter を通じて、通常応答・静的アセット・
// panic時の500にセキュリティヘッダーが付くことを検証する。
// ミドルウェア単体のテストでは検出できない、ルーターへの登録漏れを防ぐため。
func TestNewRouter_SecurityHeaders(t *testing.T) {
	t.Parallel()

	// ルーター経由の各応答に期待するヘッダーの集合。
	wantHeaders := map[string]string{
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=(), payment=()",
	}

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{
			name:       "トップページ",
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "静的アセット",
			path:       "/static/style.css",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ハンドラーがpanicしたときの500",
			path:       "/test-panic",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)
			router.Get("/test-panic", func(_ http.ResponseWriter, _ *http.Request) {
				panic("テスト用のpanic")
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			for name, want := range wantHeaders {
				if got := resp.Header.Get(name); got != want {
					t.Errorf("%s = %q、期待値 = %q", name, got, want)
				}
			}
		})
	}
}

// TestNewRouter_NotFound は経路の無いパスが共通の404ページに落ちることを固定する。
// 描画そのもののテストは internal/httperror が持つため、ここで見るのは
// r.NotFound への登録と、ロケールを載せるミドルウェアより内側で描画されることの2点。
func TestNewRouter_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantHeading string
	}{
		{
			name:        "日本語版",
			path:        "/no-such-page",
			wantHeading: "ページが見つかりません",
		},
		{
			name:        "英語版",
			path:        "/en/no-such-page",
			wantHeading: "Page not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestNewRouter_MethodNotAllowed は経路はあるがメソッドを受け付けないリクエストが、
// 共通の405ページに落ち、許可メソッドを伴って返ることを固定する。
// 描画そのもののテストは internal/httperror が持つため、ここで見るのは
// r.MethodNotAllowed への登録と、Allowの付与と、ロケールを載せるミドルウェアより内側で描画されることの3点。
func TestNewRouter_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		path        string
		wantHeading string
	}{
		{
			name:        "日本語版のトップページへのPOST",
			method:      http.MethodPost,
			path:        "/",
			wantHeading: "この操作は利用できません",
		},
		{
			name:        "英語版のトップページへのPOST",
			method:      http.MethodPost,
			path:        "/en",
			wantHeading: "This action is not available",
		},
		{
			name:        "ヘルスチェックへのDELETE",
			method:      http.MethodDelete,
			path:        "/health",
			wantHeading: "この操作は利用できません",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			req := withCSRFToken(httptest.NewRequest(tt.method, tt.path, nil))
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMethodNotAllowed)
			}

			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値 = text/htmlで始まる値", got)
			}

			// RFC 9110は405の応答に、そのアドレスが受け付けるメソッドの一覧を求めている。
			// HEADが入るのはGetHeadがGETのルートへフォールバックするため。
			wantAllow := []string{http.MethodGet, http.MethodHead}
			if got := rec.Header().Values("Allow"); !slices.Equal(got, wantAllow) {
				t.Errorf("Allow = %v、期待値 = %v", got, wantAllow)
			}

			if !strings.Contains(rec.Body.String(), tt.wantHeading) {
				t.Errorf("レスポンスボディに %qが含まれていない", tt.wantHeading)
			}
		})
	}
}

// TestAllowedMethods はAllowヘッダーの元になる許可メソッドが、ルーターの登録内容から求まることを検証する。
func TestAllowedMethods(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())
	router.Head("/head-only", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/post-only", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name string
		path string
		want []string
	}{
		// HEADが入るのはGetHeadがGETのルートへフォールバックするため。
		{name: "GETだけを登録したトップページ", path: "/", want: []string{http.MethodGet, http.MethodHead}},
		{name: "GETだけを登録した英語版のトップページ", path: "/en", want: []string{http.MethodGet, http.MethodHead}},
		{name: "GETだけを登録したヘルスチェック", path: "/health", want: []string{http.MethodGet, http.MethodHead}},
		{
			name: "メソッドを限定せず登録した静的アセット",
			path: "/static/css/style.css",
			want: allowProbeMethods,
		},
		{name: "HEADだけを登録したページ", path: "/head-only", want: []string{http.MethodHead}},
		{name: "POSTだけを登録したページ", path: "/post-only", want: []string{http.MethodPost}},
		{name: "経路の無いパス", path: "/no-such-page", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := allowedMethods(router, tt.path)

			if len(got) != len(tt.want) {
				t.Fatalf("許可メソッド = %v、期待値 = %v", got, tt.want)
			}

			for i, method := range tt.want {
				if got[i] != method {
					t.Errorf("許可メソッド[%d] = %q、期待値 = %q", i, got[i], method)
				}
			}
		})
	}
}

// TestNewRouter_Head はGETを登録したアドレスがHEADにも同じステータスで応えることを固定する。
// GetHeadによるフォールバックがルーター全体へ適用されていることを検証する。
func TestNewRouter_Head(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		path            string
		wantStatus      int
		wantContentType string
	}{
		{
			name:            "日本語版のトップページ",
			path:            "/",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
		},
		{
			name:            "英語版のトップページ",
			path:            "/en",
			wantStatus:      http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
		},
		{
			name:            "ヘルスチェック",
			path:            "/health",
			wantStatus:      http.StatusOK,
			wantContentType: "application/json",
		},
		{
			name:            "経路の無いパス",
			path:            "/no-such-page",
			wantStatus:      http.StatusNotFound,
			wantContentType: "text/html; charset=utf-8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			req := httptest.NewRequest(http.MethodHead, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			// HEADの応答はGETと同じヘッダーを持つ (RFC 9110)。
			if got := rec.Header().Get("Content-Type"); got != tt.wantContentType {
				t.Errorf("Content-Type = %q、期待値 = %q", got, tt.wantContentType)
			}
		})
	}
}

// TestNewRouter_HeadSendsNoBody はHEADの応答がボディを伴わないことを検証する。
//
// 生のコネクションへHEADを書いて応答をそのまま読む。
// httptest.NewRecorder はハンドラーが書いたボディを保持し、net/httpのHTTPクライアントは
// HEADの応答のボディを読まないため、どちらもボディが送られたかどうかを観測できない。
func TestNewRouter_HeadSendsNoBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(newRouter(testConfig(), testutil.GetTestDB(), t.TempDir()))
	t.Cleanup(server.Close)

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("接続のエラー = %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("接続のクローズのエラー = %v", err)
		}
	})

	request := "HEAD / HTTP/1.1\r\nHost: " + server.Listener.Addr().String() + "\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("リクエストの書き込みのエラー = %v", err)
	}

	raw, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("応答の読み取りのエラー = %v", err)
	}

	response := string(raw)

	if !strings.HasPrefix(response, "HTTP/1.1 200 OK\r\n") {
		t.Errorf("応答の先頭 = %q、期待値 = %q", firstLine(response), "HTTP/1.1 200 OK")
	}

	// ヘッダーの終端より後ろに何も続かないことが、ボディを送っていないこと。
	headerEnd := strings.Index(response, "\r\n\r\n")
	if headerEnd < 0 {
		t.Fatalf("応答にヘッダーの終端が無い: %q", response)
	}

	if body := response[headerEnd+len("\r\n\r\n"):]; body != "" {
		t.Errorf("レスポンスボディ = %q、期待値 = 空文字列", body)
	}

	// GETなら返すボディの長さは、HEADの応答でもContent-Lengthとして示してよい (RFC 9110)。
	if !strings.Contains(response, "Content-Type: text/html; charset=utf-8\r\n") {
		t.Errorf("応答にHTMLのContent-Typeが含まれていない: %q", response[:headerEnd])
	}
}

// firstLine は応答の1行目を返す。失敗メッセージに応答全体を並べないため。
func firstLine(response string) string {
	if i := strings.Index(response, "\r\n"); i >= 0 {
		return response[:i]
	}

	return response
}

// TestNewRouter_HeadRouting はGETへのフォールバックと明示したHEADのルートで、
// 後続へHEADのまま渡すことを検証する。ハンドラーのHEAD用の処理を有効に保つため。
func TestNewRouter_HeadRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		registerGet  bool
		registerHead bool
		wantHandler  string
	}{
		{name: "GETへのフォールバック", registerGet: true, wantHandler: "get"},
		{name: "明示したHEADを優先", registerGet: true, registerHead: true, wantHandler: "head"},
		{name: "HEADだけのルート", registerHead: true, wantHandler: "head"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())
			var gotHandler, gotMethod string
			if tt.registerGet {
				router.Get("/test-head", func(w http.ResponseWriter, r *http.Request) {
					gotHandler, gotMethod = "get", r.Method
					w.WriteHeader(http.StatusOK)
				})
			}
			if tt.registerHead {
				router.Head("/test-head", func(w http.ResponseWriter, r *http.Request) {
					gotHandler, gotMethod = "head", r.Method
					w.WriteHeader(http.StatusOK)
				})
			}
			req := httptest.NewRequest(http.MethodHead, "/test-head", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if gotHandler != tt.wantHandler {
				t.Errorf("呼ばれたハンドラー = %q、期待値 = %q", gotHandler, tt.wantHandler)
			}
			if gotMethod != http.MethodHead {
				t.Errorf("後続のメソッド = %q、期待値 = %q", gotMethod, http.MethodHead)
			}
			if req.Method != http.MethodHead {
				t.Errorf("受け取ったリクエストのメソッド = %q、期待値 = %q", req.Method, http.MethodHead)
			}
		})
	}
}

// TestNewRouter_HeadStaticAsset は静的配信がHEADで本文を書き出さないことを検証する。
// Recorderは本文を捨てないため、ネットワーク層が抑止する前のFileServerの動作を確認できる。
func TestNewRouter_HeadStaticAsset(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	content := strings.Repeat(" ", 1<<20)
	if err := os.WriteFile(filepath.Join(staticDir, "sample.css"), []byte(content), 0600); err != nil {
		t.Fatalf("静的ファイルの作成のエラー = %v", err)
	}
	router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/static/sample.css", nil))
	if get.Code != http.StatusOK || get.Body.String() != content {
		t.Fatalf("GETの静的配信が期待どおりでない: ステータス = %d、本文の長さ = %d", get.Code, get.Body.Len())
	}
	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/static/sample.css", nil))
	if head.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", head.Code, http.StatusOK)
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Last-Modified", "Accept-Ranges"} {
		if got, want := head.Header().Get(name), get.Header().Get(name); got != want {
			t.Errorf("%s = %q、期待値 = %q", name, got, want)
		}
	}
	if head.Body.Len() != 0 {
		t.Errorf("FileServerが書き出した本文の長さ = %d、期待値 = 0", head.Body.Len())
	}
}

// TestNewRouter_RedirectSlashes は末尾スラッシュ付きのURLが、スラッシュ無しの同じURLへ
// 301で正規化され、その301が発行元を名乗ることを固定する。
//
// 正規化とRedirect-Byはどちらもミドルウェア単体のテストを持つが、
// ルーターへの登録漏れや登録順の入れ替わりはそこでは検出できない。
func TestNewRouter_RedirectSlashes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path         string
		wantLocation string
	}{
		{
			name:         "ヘルスチェック",
			path:         "/health/",
			wantLocation: "/health",
		},
		{
			name:         "英語版のトップページ",
			path:         "/en/",
			wantLocation: "/en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusMovedPermanently {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if got := rec.Header().Get("Redirect-By"); got != "cutre" {
				t.Errorf("Redirect-By = %q、期待値 = %q", got, "cutre")
			}

			// 転送先が本当に応答することまで見る。Locationの文字列が合っていても、
			// 正規化した形にルートが無ければ訪問者は404に着く。
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.wantLocation, nil))

			if rec.Code != http.StatusOK {
				t.Errorf("%sのステータスコード = %d、期待値 = %d", tt.wantLocation, rec.Code, http.StatusOK)
			}
		})
	}
}

// TestNewRouter_NoRedirectForNormalizedPaths は、正規化するものが無いリクエストが
// リダイレクトされず、発行元のヘッダーも持たないことを固定する。
//
// トップページを含めるのは、パスが "/" の1文字だけで末尾スラッシュと見分けが付かず、
// 正規化の条件を誤ると自分自身へのリダイレクトで無限ループになるため。
func TestNewRouter_NoRedirectForNormalizedPaths(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	for _, path := range []string{"/", "/en", "/health", "/static/css/style.css"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Redirect-By"); got != "" {
				t.Errorf("Redirect-By = %q、期待値 = 付かないこと", got)
			}
		})
	}
}

// TestNewRouter_AssetDirectoryDoesNotLoop は、静的アセットを収めたディレクトリが
// URLの2つの形の間を往復するのではなく404に落ち着くことを固定する。
//
// http.FileServer はディレクトリのパスに末尾スラッシュを足すリダイレクトを返し、
// middleware.RedirectSlashes はそれを剥がす。2つを素で組み合わせると訪問者は無限ループを踏む。
// Cutreが免れているのは配信元がディレクトリを存在しないものとして扱うためで (assetFileSystem)、
// それが成り立たなくなったことに気付くのが本テストである。
func TestNewRouter_AssetDirectoryDoesNotLoop(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "css"), 0o755); err != nil {
		t.Fatalf("テスト用ディレクトリの作成のエラー = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "css", "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	router := newRouter(testConfig(), testutil.GetTestDB(), staticDir)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/css/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusMovedPermanently)
	}
	location := rec.Header().Get("Location")
	if location != "/static/css" {
		t.Fatalf("Location = %q、期待値 = %q", location, "/static/css")
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, location, nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("%sのステータスコード = %d、期待値 = %d", location, rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("%sのLocation = %q、期待値 = 付かないこと", location, got)
	}
}

// TestNewRouter_RedirectEscapedAssets は予約文字を含むアセットの転送先を
// 実クライアントで辿り、ファイル名とクエリが同じまま取得できることを検証する。
func TestNewRouter_RedirectEscapedAssets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filename    string
		escapedName string
	}{
		{name: "ハッシュ", filename: "a#b.css", escapedName: "a%23b.css"},
		{name: "疑問符", filename: "a?b.css", escapedName: "a%3Fb.css"},
		{name: "パーセント", filename: "a%b.css", escapedName: "a%25b.css"},
		{name: "バックスラッシュ", filename: "a\\b.css", escapedName: "a%5Cb.css"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			staticDir := t.TempDir()
			const wantBody = "body{color:red}"
			if err := os.WriteFile(filepath.Join(staticDir, tt.filename), []byte(wantBody), 0o600); err != nil {
				t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
			}
			server := httptest.NewServer(newRouter(testConfig(), testutil.GetTestDB(), staticDir))
			t.Cleanup(server.Close)

			const query = "v=1&tag=a%26b&tag=c+d"
			wantLocation := "/static/" + tt.escapedName + "?" + query
			client := server.Client()
			redirects := 0
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				redirects++
				if len(via) > 1 {
					return http.ErrUseLastResponse
				}
				if got := req.Response.StatusCode; got != http.StatusMovedPermanently {
					t.Errorf("転送ステータス = %d、期待値 = %d", got, http.StatusMovedPermanently)
				}
				if got := req.Response.Header.Get("Location"); got != wantLocation {
					t.Errorf("Location = %q、期待値 = %q", got, wantLocation)
				}
				if got := req.Response.Header.Get("Redirect-By"); got != "cutre" {
					t.Errorf("Redirect-By = %q、期待値 = cutre", got)
				}
				return nil
			}

			resp, err := client.Get(server.URL + "/static/" + tt.escapedName + "/?" + query)
			if err != nil {
				t.Fatalf("アセット取得のエラー = %v", err)
			}
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if redirects != 1 {
				t.Errorf("転送回数 = %d、期待値 = 1", redirects)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("最終ステータス = %d、期待値 = %d", resp.StatusCode, http.StatusOK)
			}
			if got, want := resp.Request.URL.Path, "/static/"+tt.filename; got != want {
				t.Errorf("最終パス = %q、期待値 = %q", got, want)
			}
			if got := resp.Request.URL.RawQuery; got != query {
				t.Errorf("最終クエリ = %q、期待値 = %q", got, query)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("本文の読み取りのエラー = %v", err)
			}
			if string(body) != wantBody {
				t.Errorf("本文 = %q、期待値 = %q", body, wantBody)
			}
		})
	}
}

// TestNewRouter_CacheControl は、ルーター経由の各応答が明示のキャッシュ方針を持ち、
// 言語設定による分岐の申告がHTMLにだけ付くことを固定する。
//
// ミドルウェア単体のテストとは別に置くのは、方針が2つのミドルウェアと
// エラーページの取り合わせで決まり、登録の順序を変えると結果が変わるため。
func TestNewRouter_CacheControl(t *testing.T) {
	t.Parallel()

	staticDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(staticDir, "style.css"), []byte("body{color:red}"), 0o600); err != nil {
		t.Fatalf("テスト用ファイルの作成のエラー = %v", err)
	}

	tests := []struct {
		name             string
		env              string
		path             string
		rangeHeader      string
		wantStatus       int
		wantCacheControl string
		wantVary         string
	}{
		{
			name:             "トップページ",
			env:              "dev",
			path:             "/",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
		{
			name:             "静的アセット",
			env:              "prod",
			path:             "/static/style.css",
			wantStatus:       http.StatusOK,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "開発環境の静的アセット",
			env:              "dev",
			path:             "/static/style.css",
			wantStatus:       http.StatusOK,
			wantCacheControl: "no-store",
		},
		{
			name:             "静的アセットの一部だけを求めるリクエスト",
			env:              "prod",
			path:             "/static/style.css",
			rangeHeader:      "bytes=0-4",
			wantStatus:       http.StatusPartialContent,
			wantCacheControl: "public, max-age=31536000, immutable",
		},
		{
			name:             "存在しないアセット",
			env:              "prod",
			path:             "/static/missing.css",
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
		},
		{
			name:             "404ページ",
			env:              "dev",
			path:             "/missing",
			wantStatus:       http.StatusNotFound,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "ログイン画面",
			env:              "dev",
			path:             "/sign_in",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "登録の画面",
			env:              "dev",
			path:             "/sign_up",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "英語版の登録の画面",
			env:              "dev",
			path:             "/en/sign_up",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "パスワードリセットの申請の画面",
			env:              "dev",
			path:             "/password_reset",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "英語版のパスワードリセットの申請を受け付けた後の画面",
			env:              "dev",
			path:             "/en/password_reset/sent",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "英語版のログイン画面",
			env:              "dev",
			path:             "/en/sign_in",
			wantStatus:       http.StatusOK,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			// 同じURLがログイン中の訪問者にはページを返すため、リダイレクトも保存させない。
			name:             "未ログインでログイン後のページを開いたときのリダイレクト",
			env:              "dev",
			path:             "/home",
			wantStatus:       http.StatusSeeOther,
			wantCacheControl: "private, no-store",
			wantVary:         "Accept-Language",
		},
		{
			name:             "末尾スラッシュの正規化",
			env:              "dev",
			path:             "/health/",
			wantStatus:       http.StatusMovedPermanently,
			wantCacheControl: "private, no-cache",
			wantVary:         "Accept-Language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := testConfig()
			cfg.Env = tt.env

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}

			rec := httptest.NewRecorder()
			newRouter(cfg, testutil.GetTestDB(), staticDir).ServeHTTP(rec, req)
			resp := rec.Result()
			t.Cleanup(func() {
				if err := resp.Body.Close(); err != nil {
					t.Errorf("レスポンスボディのクローズのエラー = %v", err)
				}
			})

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := resp.Header.Values("Cache-Control"); len(got) != 1 || got[0] != tt.wantCacheControl {
				t.Errorf("Cache-Control = %v、期待値 = [%q]", got, tt.wantCacheControl)
			}

			values := resp.Header.Values("Vary")
			if tt.wantVary == "" {
				if len(values) != 0 {
					t.Errorf("Vary = %v、期待値 = 無し", values)
				}
				return
			}
			if len(values) != 1 || values[0] != tt.wantVary {
				t.Errorf("Vary = %v、期待値 = [%q]", values, tt.wantVary)
			}
		})
	}
}

// TestNewRouter_SetUser は newRouter が middleware.SetUser をアプリケーションのルートに登録していることを固定する。
//
// あわせて、ロケールを決める middleware.I18n より先に走ることも見る。
// ログイン後のページの表示言語を users.locale で決めるには、I18nがユーザーを見られる並び順である必要がある。
func TestNewRouter_SetUser(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()

	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(userID)
	userSession.Build()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "セッションCookieを持つリクエスト", token: userSession.Token(), want: atname},
		{name: "セッションCookieを持たないリクエスト", token: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), db, t.TempDir())

			var gotAtname string
			var localeResolvedAfterUser bool
			router.Get("/test-current-user", func(_ http.ResponseWriter, r *http.Request) {
				if user := middleware.UserFromContext(r.Context()); user != nil {
					gotAtname = user.Atname
				}
				localeResolvedAfterUser = i18n.GetLocale(r.Context()) != ""
			})

			req := httptest.NewRequest(http.MethodGet, "/test-current-user", nil)
			if tt.token != "" {
				req.AddCookie(&http.Cookie{Name: session.CookieName, Value: tt.token})
			}
			router.ServeHTTP(httptest.NewRecorder(), req)

			if gotAtname != tt.want {
				t.Errorf("アットネーム = %q、期待値 = %q", gotAtname, tt.want)
			}
			if !localeResolvedAfterUser {
				t.Error("ロケールが解決されていない")
			}
		})
	}
}

// TestNewRouter_CSRF は newRouter がCSRFの検証をアプリケーションのルートに登録していることを固定する。
// ミドルウェア自体の挙動は internal/middleware のテストが持つため、ここで見るのは配線だけ。
func TestNewRouter_CSRF(t *testing.T) {
	t.Parallel()

	const token = "csrf-token"

	tests := []struct {
		name            string
		withToken       bool
		wantStatus      int
		wantHandlerCall bool
	}{
		{name: "トークンを持たない送信は届かない", withToken: false, wantStatus: http.StatusForbidden},
		{name: "トークンを持つ送信は届く", withToken: true, wantStatus: http.StatusOK, wantHandlerCall: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

			handlerCalled := false
			router.Post("/test-csrf", func(_ http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
			})

			form := url.Values{}
			if tt.withToken {
				form.Set(middleware.CSRFFieldName, token)
			}
			req := httptest.NewRequest(http.MethodPost, "/test-csrf", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if handlerCalled != tt.wantHandlerCall {
				t.Errorf("ハンドラーの到達 = %t、期待値 = %t", handlerCalled, tt.wantHandlerCall)
			}
		})
	}
}

// TestNewRouter_MethodOverride は、フォームからのPOSTが _method でDELETEのルートに届くことを固定する。
// CSRFの検証を先に通す必要があるため、トークンも載せて送る。
func TestNewRouter_MethodOverride(t *testing.T) {
	t.Parallel()

	const token = "csrf-token"

	router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

	deleted := false
	router.Delete("/test-method-override", func(_ http.ResponseWriter, _ *http.Request) {
		deleted = true
	})

	form := url.Values{
		middleware.CSRFFieldName:           {token},
		middleware.MethodOverrideFieldName: {"DELETE"},
	}
	req := httptest.NewRequest(http.MethodPost, "/test-method-override", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if !deleted {
		t.Error("DELETEのルートに到達しなかった")
	}
}

// TestNewRouter_Flash は newRouter がフラッシュメッセージの読み取りを登録していることを固定する。
func TestNewRouter_Flash(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

	var gotFlash *session.FlashMessage
	router.Get("/test-flash", func(_ http.ResponseWriter, r *http.Request) {
		gotFlash = session.FlashFromContext(r.Context())
	})

	data, err := json.Marshal(session.FlashMessage{Type: session.FlashSuccess, Message: "ログインしました"})
	if err != nil {
		t.Fatalf("テスト用フラッシュメッセージの組み立てのエラー = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/test-flash", nil)
	req.AddCookie(&http.Cookie{
		Name:  session.FlashCookieName,
		Value: base64.RawURLEncoding.EncodeToString(data),
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if gotFlash == nil {
		t.Fatal("フラッシュメッセージ = nil、非nilを期待")
	}
	if gotFlash.Message != "ログインしました" {
		t.Errorf("メッセージ = %q、期待値 = %q", gotFlash.Message, "ログインしました")
	}
}

// uniqueRemoteAddr は実行ごとに異なる接続元のアドレスを返す。
// ログインのレート制限はデータベースに数えを残すため、固定のアドレスだと
// テストを繰り返し実行したときに上限へ達する。IPv6は /64 単位で数えるため、その範囲を乱数で選ぶ。
func uniqueRemoteAddr(t *testing.T) string {
	t.Helper()

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("乱数の生成のエラー = %v", err)
	}
	return fmt.Sprintf("[2001:db8:%x:%x:%x::1]:12345", b[0:2], b[2:4], b[4:6])
}

// TestNewRouter_SignIn は、ログインの送信からホームの表示までを配線ごと通して検証する。
// ログインした利用者はユーザーの言語でホームとフラッシュメッセージを見て、
// ログイン前向けのページを開くとホームへ送られる。
func TestNewRouter_SignIn(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())
	remoteAddr := uniqueRemoteAddr(t)

	// ログイン前の画面の言語 (日本語) とユーザーの言語 (英語) を違えて、ホームがユーザーの言語で出ることを確かめる。
	email := testutil.UniqueEmail("router-sign-in")
	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, db).WithEmail(email).WithAtname(atname).WithLocale(model.LocaleEn).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()

	// 失敗時の再描画は入力したメールアドレスを埋めて返すため、保存させない。
	form := url.Values{"email": {email}, "password": {"wrong-password"}}
	req := withCSRFToken(httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ログイン失敗の応答 = %d、期待値 = %d\n%s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("ログイン失敗の応答のCache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	form = url.Values{"email": {email}, "password": {"password123"}}
	req = withCSRFToken(httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
		t.Fatalf("ログインの応答 = %d %q、期待値 = 303 %q\n%s", rec.Code, rec.Header().Get("Location"), "/home", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("ログインの応答のCache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	var cookies []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName || c.Name == session.FlashCookieName {
			cookies = append(cookies, c)
		}
	}
	if len(cookies) != 2 {
		t.Fatalf("セッションとフラッシュのCookie = %d件、期待値 = 2件", len(cookies))
	}

	req = httptest.NewRequest(http.MethodGet, "/home", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ホームのステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	// ログアウト後に戻る操作で、保存されたホームが再表示されないようにする。
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("ホームのCache-Control = %q、期待値 = %q", got, "private, no-store")
	}
	body := rec.Body.String()
	for _, want := range []string{`<html lang="en">`, "Welcome, @" + atname, "You&#39;re signed in"} {
		if !strings.Contains(body, want) {
			t.Errorf("ホームに %q が含まれていない", want)
		}
	}

	// ログイン前向けのページは、ログイン済みの訪問者をホームへ送る。
	for _, path := range []string{"/", "/en", "/sign_in", "/en/sign_in"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookies[0])
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
			t.Errorf("ログイン済みで %s を開いたときの応答 = %d %q、期待値 = 303 %q", path, rec.Code, rec.Header().Get("Location"), "/home")
		}
	}

	// 同じURLが未ログインの訪問者にはログイン画面を返すため、ログイン画面のリダイレクトも保存させない。
	for _, path := range []string{"/sign_in", "/en/sign_in"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		req.AddCookie(cookies[0])
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("ログイン済みで %s を開いたときのCache-Control = %q、期待値 = %q", path, got, "private, no-store")
		}
	}
}

// TestNewRouter_SignInTwoFactor は、二要素認証を有効にしたユーザーがパスワードだけではセッションを得られず、
// 認証アプリのコードを入力して初めてホームを開けることを、配線ごと通して検証する。
func TestNewRouter_SignInTwoFactor(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	cfg := testConfig()
	router := newRouter(cfg, db, t.TempDir())
	remoteAddr := uniqueRemoteAddr(t)

	email := testutil.UniqueEmail("router-sign-in-two-factor")
	userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).WithPassword("password123").Build()
	twoFactorKey, err := auth.NewTwoFactorKey(cfg.TOTPEncryptionKey)
	if err != nil {
		t.Fatalf("鍵の作成のエラー = %v", err)
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("秘密鍵の生成のエラー = %v", err)
	}
	id := uuid.UUID(userID)
	ciphertext, err := twoFactorKey.EncryptTOTPSecret(secret, id[:])
	if err != nil {
		t.Fatalf("秘密鍵の暗号化のエラー = %v", err)
	}
	testutil.NewUserTwoFactorAuthBuilder(t, db, userID).WithSecretCiphertext(ciphertext).WithEnabledAt(time.Now()).Build()

	form := url.Values{"email": {email}, "password": {"password123"}, "return_to": {"/settings/two_factor_auth"}}
	req := withCSRFToken(httptest.NewRequest(http.MethodPost, "/en/sign_in", strings.NewReader(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	wantLocation := "/en/sign_in/two_factor?return_to=%2Fsettings%2Ftwo_factor_auth"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != wantLocation {
		t.Fatalf("パスワードの送信の応答 = %d %q、期待値 = 303 %q\n%s", rec.Code, rec.Header().Get("Location"), wantLocation, rec.Body.String())
	}
	var pendingCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case session.CookieName:
			t.Fatal("パスワードだけでセッションCookieが発行された")
		case session.TwoFactorPendingCookieName:
			pendingCookie = c
		}
	}
	if pendingCookie == nil {
		t.Fatal("コードの入力を待つCookieが発行されていない")
	}

	// コードを入力する前は、コードの入力を待つCookieを持っていてもログイン後のページを開けない。
	req = httptest.NewRequest(http.MethodGet, "/home", nil)
	req.AddCookie(pendingCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_in?return_to=%2Fhome" {
		t.Errorf("コードの入力前のホームの応答 = %d %q、ログイン画面へのリダイレクトを期待", rec.Code, rec.Header().Get("Location"))
	}

	req = httptest.NewRequest(http.MethodGet, wantLocation, nil)
	req.AddCookie(pendingCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("コードの入力画面のステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("コードの入力画面のCache-Control = %q、期待値 = %q", got, "private, no-store")
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}
	form = url.Values{"code": {code}, "return_to": {"/settings/two_factor_auth"}}
	req = withCSRFToken(httptest.NewRequest(http.MethodPost, "/en/sign_in/two_factor", strings.NewReader(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	req.AddCookie(pendingCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/two_factor_auth" {
		t.Fatalf("コードの送信の応答 = %d %q、期待値 = 303 %q\n%s", rec.Code, rec.Header().Get("Location"), "/settings/two_factor_auth", rec.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("コードが合ったのにセッションCookieが発行されていない")
	}

	req = httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ログイン後の二要素認証の画面のステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
}

// TestNewRouter_SignInTwoFactorRecovery は画面への到達とコードの使い切りまでをルーターから確かめる。
func TestNewRouter_SignInTwoFactorRecovery(t *testing.T) {
	t.Parallel()
	db := testutil.GetTestDB()
	cfg := testConfig()
	router := newRouter(cfg, db, t.TempDir())
	userID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db, userID).WithEnabledAt(time.Now()).Build()
	key, err := auth.NewTwoFactorKey(cfg.TOTPEncryptionKey)
	if err != nil {
		t.Fatalf("鍵の作成: %v", err)
	}
	codeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	const code = "abcd-2345"
	if err := codeRepo.CreateAll(t.Context(), userID, []string{key.RecoveryCodeDigest(auth.NormalizeRecoveryCode(code))}); err != nil {
		t.Fatalf("リカバリーコードの作成: %v", err)
	}
	pendingRec := httptest.NewRecorder()
	session.NewContinuationManager(cfg.ContinuationTokenKey).SetTwoFactorPendingUserID(pendingRec, userID)
	pendingCookie := pendingRec.Result().Cookies()[0]

	for _, prefix := range []string{"", "/en"} {
		path := prefix + "/sign_in/two_factor/recovery"
		req := httptest.NewRequest(http.MethodGet, path+"?return_to=%2Fhome", nil)
		req.AddCookie(pendingCookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d、200を期待", path, rec.Code)
		}
		for _, want := range []string{`action="` + path + `"`, `name="return_to" value="/home"`, `href="` + prefix + `/sign_in/two_factor?return_to=%2Fhome"`, `autocomplete="off"`, `content="noindex`} {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("GET %s の本文に %q が無い", path, want)
			}
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("GET %s のCache-Control = %q", path, got)
		}
	}

	path := "/sign_in/two_factor/recovery"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_in" {
		t.Errorf("Cookie無しのGET = %d %q、ログインへ戻ることを期待", rec.Code, rec.Header().Get("Location"))
	}

	post := func(value string) *httptest.ResponseRecorder {
		form := url.Values{"code": {value}, "return_to": {"/home"}}
		req := withCSRFToken(httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode())))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(pendingCookie)
		req.RemoteAddr = uniqueRemoteAddr(t)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	sessionCookie := func(rec *httptest.ResponseRecorder) *http.Cookie {
		for _, c := range rec.Result().Cookies() {
			if c.Name == session.CookieName {
				return c
			}
		}
		return nil
	}
	rec = post("wrong")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `aria-invalid="true"`) {
		t.Errorf("不正なコード = %d、入力欄のエラーを期待", rec.Code)
	}
	rec = post("ABCD - 2345")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
		t.Fatalf("正しいコード = %d %q、ホームへの303を期待", rec.Code, rec.Header().Get("Location"))
	}
	if c := sessionCookie(rec); c == nil || c.Value == "" {
		t.Error("セッションCookieが発行されていない")
	}
	if count, err := codeRepo.CountUnused(t.Context(), userID); err != nil || count != 0 {
		t.Errorf("未使用コードの数 = (%d, %v)、0を期待", count, err)
	}
	rec = post(code)
	if rec.Code != http.StatusUnprocessableEntity || sessionCookie(rec) != nil {
		t.Errorf("使用済みコード = %d、セッション無しの422を期待", rec.Code)
	}
}

// TestNewRouter_SignInRoutes は、ログイン画面を言語版ごとに配信し、
// 未ログインでホームを開くとログイン画面へ戻り先付きで送ることを検証する。
func TestNewRouter_SignInRoutes(t *testing.T) {
	t.Parallel()

	router := newRouter(testConfig(), testutil.GetTestDB(), t.TempDir())

	tests := []struct {
		path         string
		wantStatus   int
		wantLocation string
		wantBody     string
	}{
		{path: "/sign_in", wantStatus: http.StatusOK, wantBody: `action="/sign_in"`},
		{path: "/en/sign_in", wantStatus: http.StatusOK, wantBody: `action="/en/sign_in"`},
		// パスワードを確かめていなければ、コードの入力画面は同じ言語版のログイン画面へ送る。
		{path: "/sign_in/two_factor", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in"},
		{path: "/en/sign_in/two_factor", wantStatus: http.StatusSeeOther, wantLocation: "/en/sign_in"},
		{path: "/home", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2Fhome"},
		{path: "/@cutre_user", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2F%40cutre_user"},
		{path: "/settings/invitation", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2Fsettings%2Finvitation"},
		{path: "/settings/two_factor_auth", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2Fsettings%2Ftwo_factor_auth"},
		{path: "/settings/two_factor_auth/new", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2Fsettings%2Ftwo_factor_auth%2Fnew"},
		{path: "/settings/withdrawal", wantStatus: http.StatusSeeOther, wantLocation: "/sign_in?return_to=%2Fsettings%2Fwithdrawal"},
		// ログイン後のページは言語版を持たない。
		{path: "/en/home", wantStatus: http.StatusNotFound},
		{path: "/en/@cutre_user", wantStatus: http.StatusNotFound},
		{path: "/en/settings/invitation", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("レスポンスボディに %q が含まれていない", tt.wantBody)
			}
		})
	}
}

// TestNewRouter_SignOut は、プロフィールのログアウトのフォームと同じ送信 (_method でDELETEに上書きしたPOST) で
// ログアウトでき、その後は同じセッションCookieでホームを開けないことを検証する。
func TestNewRouter_SignOut(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())

	userID := testutil.NewUserBuilder(t, db).Build()
	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(userID)
	userSession.Build()
	sessionCookie := &http.Cookie{Name: session.CookieName, Value: userSession.Token()}

	form := url.Values{middleware.MethodOverrideFieldName: {http.MethodDelete}}
	req := withCSRFToken(httptest.NewRequest(http.MethodPost, "/user_session", strings.NewReader(form.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("ログアウトの応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/")
	}

	// Cookieが消えずに残っていても、セッションの行が無いためログインとして扱われない。
	req = httptest.NewRequest(http.MethodGet, "/home", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/sign_in") {
		t.Errorf("ログアウト後にホームを開いたときの応答 = %d %q、ログイン画面へのリダイレクトを期待", rec.Code, rec.Header().Get("Location"))
	}
}

// TestNewRouter_SettingsInvitation は、ログイン中のユーザーが招待の画面を開けて、
// 招待リンクと作り直しのフォームのCSRFトークンを載せた応答を保存させないことを検証する。
func TestNewRouter_SettingsInvitation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())

	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(testutil.NewUserBuilder(t, db).Build())
	userSession.Build()
	req := withCSRFToken(httptest.NewRequest(http.MethodGet, "/settings/invitation", nil))
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Errorf("Cache-Control = %q、期待値 = private, no-store", got)
	}
	if !strings.Contains(rec.Body.String(), "/i/") {
		t.Error("レスポンスボディに招待リンクが含まれていない")
	}
	if !strings.Contains(rec.Body.String(), `name="csrf_token" value="csrf-token"`) {
		t.Error("作り直しのフォームにCSRFトークンが含まれていない")
	}
}

// TestNewRouter_SettingsInvitationRecreate は、招待リンクの作り直しがCSRFトークンとログインを求め、
// 作り直したら招待の画面へ戻すことを検証する。
func TestNewRouter_SettingsInvitationRecreate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())

	userID := testutil.NewUserBuilder(t, db).Build()
	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(userID)
	userSession.Build()
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(userID).Build()

	tests := []struct {
		name         string
		signedIn     bool
		csrf         bool
		wantCode     int
		wantLocation string
	}{
		{name: "CSRFトークンが無ければ拒む", signedIn: true, csrf: false, wantCode: http.StatusForbidden},
		{name: "ログインしていなければログイン画面へ送る", signedIn: false, csrf: true, wantCode: http.StatusSeeOther, wantLocation: "/sign_in"},
		{name: "作り直して招待の画面へ戻す", signedIn: true, csrf: true, wantCode: http.StatusSeeOther, wantLocation: "/settings/invitation"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/settings/invitation", strings.NewReader("invitation_id="+currentID.String()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if tt.csrf {
			req = withCSRFToken(req)
		}
		if tt.signedIn {
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != tt.wantCode || rec.Header().Get("Location") != tt.wantLocation {
			t.Errorf("%s: 応答 = %d %q、期待値 = %d %q", tt.name, rec.Code, rec.Header().Get("Location"), tt.wantCode, tt.wantLocation)
		}
	}

	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(context.Background(), userID)
	if err != nil || current == nil || current.ID == currentID {
		t.Errorf("取り消していない招待 = (%+v, %v)、作り直した1回分の新しい招待を期待", current, err)
	}
}

// TestNewRouter_SettingsTwoFactorAuth は、ログイン中のユーザーが登録の画面を開いて二要素認証を有効にでき、
// 有効にする送信がCSRFトークンを求めること、リカバリーコードを載せた応答を保存させないことを検証する。
func TestNewRouter_SettingsTwoFactorAuth(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())

	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(testutil.NewUserBuilder(t, db).Build())
	userSession.Build()
	sessionCookie := &http.Cookie{Name: session.CookieName, Value: userSession.Token()}

	req := withCSRFToken(httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil))
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("登録の画面のステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	match := regexp.MustCompile(`data-copy-text="([A-Z2-7]+)"`).FindStringSubmatch(rec.Body.String())
	if match == nil {
		t.Fatal("登録の画面に手入力用のキーが含まれていない")
	}
	code, err := totp.GenerateCode(match[1], time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}

	for _, withCSRF := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader("code="+code))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if withCSRF {
			req = withCSRFToken(req)
		}
		req.AddCookie(sessionCookie)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if !withCSRF {
			if rec.Code != http.StatusForbidden {
				t.Errorf("CSRFトークンの無い送信のステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
			}
			continue
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("有効にする送信のステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("Cache-Control = %q、期待値 = private, no-store", got)
		}
		if !strings.Contains(rec.Body.String(), "cutre-recovery-codes.txt") {
			t.Error("レスポンスボディにリカバリーコードの保存のボタンが含まれていない")
		}
	}
}

// TestNewRouter_InvitationAcceptance は、招待リンクの受け取りを言語版ごとに配線し、
// 登録を始めると言語版の登録の画面へ送ること、ログイン済みならホームへ送ること、保存させないことを検証する。
func TestNewRouter_InvitationAcceptance(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())
	remoteAddr := uniqueRemoteAddr(t)

	for _, prefix := range []string{"", "/en"} {
		invitation := testutil.NewInvitationBuilder(t, db)
		invitation.Build()
		path := prefix + "/i/" + invitation.Token()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("GET %s のステータスコード = %d、期待値 = %d", path, rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("GET %s のCache-Control = %q、期待値 = %q", path, got, "private, no-store")
		}
		if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("GET %s のReferrer-Policy = %q、期待値 = no-referrer", path, got)
		}

		req = withCSRFToken(httptest.NewRequest(http.MethodPost, path, nil))
		req.RemoteAddr = remoteAddr
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		wantLocation := prefix + "/sign_up"
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != wantLocation {
			t.Errorf("POST %s の応答 = %d %q、期待値 = 303 %q", path, rec.Code, rec.Header().Get("Location"), wantLocation)
		}
		if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("POST %s のReferrer-Policy = %q、期待値 = no-referrer", path, got)
		}
	}

	// 無効な招待とレート制限の応答にも、トークンを参照元へ残さない方針を付ける。
	invalidPath := "/i/no-such-invitation"
	for i := range 26 {
		req := httptest.NewRequest(http.MethodGet, invalidPath, nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("無効な招待の%d回目の応答 = %d、期待値 = 404", i+1, rec.Code)
		}
		if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("404のReferrer-Policy = %q、期待値 = no-referrer", got)
		}
	}

	req := httptest.NewRequest(http.MethodGet, invalidPath, nil)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("レート制限の応答 = %d、期待値 = 429", rec.Code)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("429のReferrer-Policy = %q、期待値 = no-referrer", got)
	}

	// 登録済みの利用者には招待が要らないため、ログイン前向けの他のページと同じくホームへ送る。
	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(testutil.NewUserBuilder(t, db).Build())
	userSession.Build()
	invitation := testutil.NewInvitationBuilder(t, db)
	invitation.Build()

	req = httptest.NewRequest(http.MethodGet, "/i/"+invitation.Token(), nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
		t.Errorf("ログイン済みで招待リンクを開いたときの応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/home")
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("ログイン済みのリダイレクトのReferrer-Policy = %q、期待値 = no-referrer", got)
	}
}

// TestNewRouter_Password は、新しいパスワードの設定を言語版ごとに配線し、リンクのトークンをCookieへ移してから
// フォームを描画し、_method で送るPATCHでログイン画面へ送ることを固定する。
// どの応答も、トークンを参照元へ残さず、HTTPキャッシュに保存させないことも確かめる。
func TestNewRouter_Password(t *testing.T) {
	t.Parallel()

	const csrfToken = "csrf-token"
	db := testutil.GetTestDB()
	router := newRouter(testConfig(), db, t.TempDir())

	assertHeaders := func(label string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s のReferrer-Policy = %q、期待値 = no-referrer", label, got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Errorf("%s のCache-Control = %q、期待値 = %q", label, got, "private, no-store")
		}
	}

	for _, prefix := range []string{"", "/en"} {
		userID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()
		token := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(userID)
		token.Build()
		path := prefix + "/password"

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?token="+token.Token(), nil))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != path {
			t.Fatalf("リンクを開いたときの応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), path)
		}
		assertHeaders("GET "+path+"?token=", rec)
		cookies := rec.Result().Cookies()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s のステータス = %d、期待値 = 200", path, rec.Code)
		}
		assertHeaders("GET "+path, rec)

		form := url.Values{middleware.CSRFFieldName: {csrfToken}, "_method": {http.MethodPatch}, "password": {"new-password1234"}}
		req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		wantLocation := prefix + "/sign_in"
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != wantLocation {
			t.Errorf("PATCH %s の応答 = %d %q、期待値 = 303 %q", path, rec.Code, rec.Header().Get("Location"), wantLocation)
		}
		assertHeaders("PATCH "+path, rec)
	}

	// ログイン済みの利用者は、ログイン前向けの他のページと同じくホームへ送る。
	userID := testutil.NewUserBuilder(t, db).Build()
	userSession := testutil.NewUserSessionBuilder(t, db).WithUserID(userID)
	userSession.Build()
	token := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(userID)
	token.Build()

	req := httptest.NewRequest(http.MethodGet, "/password?token="+token.Token(), nil)
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: userSession.Token()})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
		t.Errorf("ログイン済みでリンクを開いたときの応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/home")
	}
	assertHeaders("ログイン済みのリダイレクト", rec)
}
