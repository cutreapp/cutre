package sign_in_two_factor_recovery_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// postCreate はリカバリーコードの送信を、表示中の言語とパスワードを確かめたCookieを載せて組み立てる。
// cookie がnilなら載せない。接続元は remoteAddr にする。
func postCreate(lang string, form url.Values, cookie *http.Cookie, remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, i18n.LocalePath(lang, "/sign_in/two_factor/recovery"), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = remoteAddr
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req.WithContext(i18n.SetLocale(req.Context(), lang))
}

// findCookie は応答が設定したCookieをnameで探す。無ければnilを返す。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Success は、コードが合えばセッションを発行し、コードの入力を待つCookieを消して、
// ユーザーの言語のフラッシュメッセージを付けて行き先へ303で送ることを、日英の言語版で検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		lang         string
		returnTo     string
		wantLocation string
	}{
		{name: "日本語で戻り先が無ければホームへ送る", lang: i18n.LangJa, returnTo: "", wantLocation: "/home"},
		{name: "英語で安全な戻り先へ送る", lang: i18n.LangEn, returnTo: "/settings/invitation", wantLocation: "/settings/invitation"},
		{name: "別のオリジンを指す戻り先は捨ててホームへ送る", lang: i18n.LangJa, returnTo: "https://evil.example.com/", wantLocation: "/home"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, db := newHandler(t)
			userID := createTwoFactorUser(t, db, model.LocaleEn, "abcd-2345")

			rec := httptest.NewRecorder()
			handler.Create(rec, postCreate(tt.lang, url.Values{"code": {"ABCD 2345"}, "return_to": {tt.returnTo}}, pendingCookie(userID), uniqueRemoteAddr(t)))

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d\n%s", rec.Code, http.StatusSeeOther, rec.Body.String())
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if c := findCookie(rec, session.CookieName); c == nil || c.Value == "" {
				t.Error("セッションCookieが発行されていない")
			}
			if c := findCookie(rec, session.TwoFactorPendingCookieName); c == nil || c.MaxAge >= 0 {
				t.Errorf("コードの入力を待つCookie = %+v、負のMaxAgeでの削除を期待", c)
			}
			if c := findCookie(rec, session.FlashCookieName); c == nil || c.Value == "" {
				t.Error("フラッシュCookieが発行されていない")
			}
		})
	}
}

// TestCreate_Rejected は、受け付けないコードをセッションを発行せずに入力欄のエラーとして再描画し、
// 入力したコードと戻り先を戻すことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	handler, db := newHandler(t)
	userID := createTwoFactorUser(t, db, model.LocaleJa, "bcde-2345")
	cookie := pendingCookie(userID)

	// 1回目で使ったコードを送り直す。
	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {"bcde-2345"}}, cookie, uniqueRemoteAddr(t)))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("1回目のステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	tests := []struct {
		name    string
		code    string
		wantMsg string
	}{
		{name: "使用済みのコード", code: "bcde-2345", wantMsg: "コードが正しくありません"},
		{name: "誤ったコード", code: "cdef-2345", wantMsg: "コードが正しくありません"},
		{name: "形式の誤り", code: "abcd-234", wantMsg: "リカバリーコードは8文字の英数字で入力してください"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {tt.code}, "return_to": {"/home"}}, cookie, uniqueRemoteAddr(t)))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusUnprocessableEntity)
			continue
		}
		body := rec.Body.String()
		for _, want := range []string{
			`aria-invalid="true" aria-describedby="code-error-0"`,
			`<p id="code-error-0" role="alert">` + tt.wantMsg,
			`value="` + tt.code + `"`,
			`name="return_to" value="/home"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, want)
			}
		}
		if c := findCookie(rec, session.CookieName); c != nil {
			t.Errorf("%s: 受け付けないコードでセッションCookieが発行された", tt.name)
		}
	}
}

// TestCreate_WithoutPendingCookie は、パスワードを確かめたCookieが無ければ、コードを消費せず
// 表示中の言語版のログイン画面へ戻り先を引き継いで送ることを検証する。
func TestCreate_WithoutPendingCookie(t *testing.T) {
	t.Parallel()

	handler, db := newHandler(t)
	userID := createTwoFactorUser(t, db, model.LocaleJa, "cdef-2345")

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(i18n.LangEn, url.Values{"code": {"cdef-2345"}, "return_to": {"/home"}}, nil, uniqueRemoteAddr(t)))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/en/sign_in?return_to=%2Fhome" {
		t.Errorf("応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/en/sign_in?return_to=%2Fhome")
	}
	if c := findCookie(rec, session.CookieName); c != nil {
		t.Error("Cookieの無い送信でセッションCookieが発行された")
	}
	if count, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CountUnused(t.Context(), userID); err != nil || count != 1 {
		t.Errorf("未使用のコードの数 = (%d, %v)、消費されていない1件を期待", count, err)
	}
}

// TestCreate_TwoFactorAuthGone は、パスワードを確かめた後に二要素認証が有効でなくなったときに、
// コードの入力を待つCookieを消してログイン画面へ送り、パスワードの確認からやり直させることを検証する。
func TestCreate_TwoFactorAuthGone(t *testing.T) {
	t.Parallel()

	handler, db := newHandler(t)
	userID := testutil.NewUserBuilder(t, db).Build()

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {"abcd-2345"}}, pendingCookie(userID), uniqueRemoteAddr(t)))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_in" {
		t.Errorf("応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/sign_in")
	}
	if c := findCookie(rec, session.TwoFactorPendingCookieName); c == nil || c.MaxAge >= 0 {
		t.Errorf("コードの入力を待つCookie = %+v、負のMaxAgeでの削除を期待", c)
	}
	if c := findCookie(rec, session.CookieName); c != nil {
		t.Error("二要素認証の無いユーザーにセッションCookieが発行された")
	}
}

// TestCreate_RateLimited は、同じユーザーへの試行が上限を超えると、IPアドレスを変えても
// 正しいコードを消費せずに429とRetry-Afterで応え、次に試せるまでの分数を示すことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	handler, db := newHandler(t)
	userID := createTwoFactorUser(t, db, model.LocaleJa, "defg-2345")
	cookie := pendingCookie(userID)

	// ユーザーの上限 (5回) まで、IPアドレスを変えながら誤ったコードで試す。
	for range 5 {
		rec := httptest.NewRecorder()
		handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {"zzzz-2345"}}, cookie, uniqueRemoteAddr(t)))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("上限内の試行のステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {"defg-2345"}}, cookie, uniqueRemoteAddr(t)))

	assertRateLimited(t, rec)
	if count, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CountUnused(t.Context(), userID); err != nil || count != 1 {
		t.Errorf("未使用のコードの数 = (%d, %v)、消費されていない1件を期待", count, err)
	}
}

// TestCreate_SharesRateLimitWithTOTP は、認証アプリのコードの試行とリカバリーコードの試行を
// 同じユーザーの上限で数え、2つの画面を行き来して回数を増やせないことを検証する。
func TestCreate_SharesRateLimitWithTOTP(t *testing.T) {
	t.Parallel()

	handler, db := newHandler(t)
	userID := createTwoFactorUser(t, db, model.LocaleJa, "efgh-2345")
	cookie := pendingCookie(userID)

	userSessionRepo := repository.NewUserSessionRepository(db)
	totpHandler := sign_in_two_factor.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(testContinuationKey),
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewCreateSignInTwoFactorUsecase(
			newTwoFactorKey(t),
			validator.NewSignInTwoFactorCreateValidator(),
			repository.NewUserRepository(db),
			repository.NewUserTwoFactorAuthRepository(db),
		),
		usecase.NewCreateSessionUsecase(userSessionRepo),
	)

	// 認証アプリのコードの画面で、ユーザーの上限 (5回) まで形式の誤ったコードを送る。
	for range 5 {
		req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(url.Values{"code": {"12a456"}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = uniqueRemoteAddr(t)
		req.AddCookie(cookie)
		req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
		rec := httptest.NewRecorder()
		totpHandler.Create(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("認証アプリのコードの試行のステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(i18n.LangJa, url.Values{"code": {"efgh-2345"}}, cookie, uniqueRemoteAddr(t)))

	assertRateLimited(t, rec)
}

// assertRateLimited は、応答が429とRetry-Afterで、次に試せるまでの分数を示し、セッションを発行していないことを確かめる。
func assertRateLimited(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter <= 0 || retryAfter > int((15*time.Minute).Seconds()) {
		t.Errorf("Retry-After = %q、1〜900秒を期待", rec.Header().Get("Retry-After"))
	}
	wantMinutes := (retryAfter + 59) / 60
	if want := "試行の回数が上限に達しました。あと" + strconv.Itoa(wantMinutes) + "分で試せます"; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("レスポンスボディに %q が含まれていない", want)
	}
	if c := findCookie(rec, session.CookieName); c != nil {
		t.Error("上限を超えた試行でセッションCookieが発行された")
	}
}
