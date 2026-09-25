package password_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/password"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

const continuationKey = "test-continuation-token-key-0123456789"

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のデータベースに直接書き込む Handler を組み立てる。
// パスワードの設定のUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler() *password.Handler {
	db := testutil.GetTestDB()
	tokenRepo := repository.NewPasswordResetTokenRepository(db)

	return password.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(continuationKey),
		session.NewFlashManager(),
		usecase.NewGetPasswordResetTokenUsecase(tokenRepo),
		usecase.NewGetPasswordResetTokenByIDUsecase(tokenRepo, repository.NewUserRepository(db)),
		usecase.NewUpdatePasswordUsecase(
			db,
			tokenRepo,
			validator.NewPasswordUpdateValidator(),
			repository.NewUserPasswordRepository(db),
			repository.NewUserSessionRepository(db),
		),
	)
}

// resetToken はパスワードを持つユーザーと、その期限内のトークンを作る。
func resetToken(t *testing.T) (*testutil.PasswordResetTokenBuilder, model.PasswordResetTokenID, string) {
	t.Helper()

	db := testutil.GetTestDB()
	email := testutil.UniqueEmail("password-handler")
	userID := testutil.NewUserBuilder(t, db).WithEmail(email).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(userID).Build()
	token := testutil.NewPasswordResetTokenBuilder(t, db).WithUserID(userID)

	return token, token.Build(), email
}

// tokenCookies はトークンのIDを運ぶCookieを返す。
func tokenCookies(id model.PasswordResetTokenID) []*http.Cookie {
	rec := httptest.NewRecorder()
	session.NewContinuationManager(continuationKey).SetPasswordResetTokenID(rec, id)

	return rec.Result().Cookies()
}

// newRequest はロケールとCookieを載せたリクエストを返す。
func newRequest(method, target string, form url.Values, locale string, cookies []*http.Cookie) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// responseCookie は応答が書き込んだ指定の名前のCookieを返す。無ければnilを返す。
func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}

// assertUnusable は、フォームの代わりに新しいリンクの申請を案内して404で応え、トークンのCookieを消したことを確かめる。
func assertUnusable(t *testing.T, name string, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusNotFound {
		t.Errorf("%s: ステータス = %d、期待値 = 404", name, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "このリンクは使えません") || !strings.Contains(body, `href="/password_reset"`) {
		t.Errorf("%s: 使えないリンクの案内と申請の画面へのリンクが含まれていない", name)
	}
	if strings.Contains(body, `name="password"`) {
		t.Errorf("%s: 使えないリンクでパスワードの入力欄が描画されている", name)
	}
	if cookie := responseCookie(rec, session.PasswordResetCookieName); cookie == nil || cookie.MaxAge >= 0 {
		t.Errorf("%s: %s = %+v、削除を期待", name, session.PasswordResetCookieName, cookie)
	}
}

// TestEdit_AcceptsToken は、リンクのトークンをCookieへ移し、表示中の言語版のトークンを持たない画面へ送ることを検証する。
func TestEdit_AcceptsToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target       string
		locale       string
		wantLocation string
	}{
		{target: "/password", locale: i18n.LangJa, wantLocation: "/password"},
		{target: "/en/password", locale: i18n.LangEn, wantLocation: "/en/password"},
	}
	for _, tt := range tests {
		token, id, _ := resetToken(t)

		rec := httptest.NewRecorder()
		newHandler().Edit(rec, newRequest(http.MethodGet, tt.target+"?token="+token.Token(), nil, tt.locale, nil))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q", tt.target, rec.Code, rec.Header().Get("Location"), tt.wantLocation)
		}
		req := newRequest(http.MethodGet, tt.wantLocation, nil, tt.locale, rec.Result().Cookies())
		if got, ok := session.NewContinuationManager(continuationKey).PasswordResetTokenID(req); !ok || got != id {
			t.Errorf("%s: CookieのトークンのID = (%v, %t)、期待値 = (%v, true)", tt.target, got, ok, id)
		}
	}
}

// TestEdit_UnusableToken は、期限切れ・無いトークンのリンクでは、Cookieを発行せずに案内を404で示すことを検証する。
func TestEdit_UnusableToken(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	expired := testutil.NewPasswordResetTokenBuilder(t, db).
		WithUserID(testutil.NewUserBuilder(t, db).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expired.Build()

	for _, token := range []string{expired.Token(), "no-such-token"} {
		rec := httptest.NewRecorder()
		newHandler().Edit(rec, newRequest(http.MethodGet, "/password?token="+token, nil, i18n.LangJa, nil))

		assertUnusable(t, token, rec)
	}
}

// TestEdit_EmptyTokenWithCookie は、空のトークンを持つリンクを開いたとき、
// 有効な継続Cookieが残っていてもリンクを使えないものとして扱うことを検証する。
func TestEdit_EmptyTokenWithCookie(t *testing.T) {
	t.Parallel()

	_, id, _ := resetToken(t)
	rec := httptest.NewRecorder()
	newHandler().Edit(rec, newRequest(http.MethodGet, "/password?token=", nil, i18n.LangJa, tokenCookies(id)))

	assertUnusable(t, "空のトークンと有効なCookie", rec)
}

// TestEdit は、Cookieのトークンで、アカウントのメールアドレスをログインの識別子として示したフォームを描画し、
// 入力補助の属性とnoindexを付けることを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	_, id, email := resetToken(t)

	rec := httptest.NewRecorder()
	newHandler().Edit(rec, newRequest(http.MethodGet, "/en/password", nil, i18n.LangEn, tokenCookies(id)))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータス = %d、期待値 = 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`action="/en/password"`,
		`name="_method" value="PATCH"`,
		`value="` + email + `" autocomplete="username" readonly`,
		`autocomplete="new-password"`,
		`data-clear-on-history-restore`,
		`<meta name="robots" content="noindex">`,
		"Set a new password",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestEdit_WithoutUsableCookie は、Cookieが無い・使えないトークンを指すときに、案内を404で示すことを検証する。
func TestEdit_WithoutUsableCookie(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	expiredID := testutil.NewPasswordResetTokenBuilder(t, db).
		WithUserID(testutil.NewUserBuilder(t, db).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute)).
		Build()

	tests := []struct {
		name    string
		cookies []*http.Cookie
	}{
		{name: "Cookie無し"},
		{name: "期限切れのトークン", cookies: tokenCookies(expiredID)},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		newHandler().Edit(rec, newRequest(http.MethodGet, "/password", nil, i18n.LangJa, tt.cookies))

		assertUnusable(t, tt.name, rec)
	}
}
