package sign_in_test

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

const testPassword = "password123"

// createUser はログインできるユーザーを作り、そのメールアドレスを返す。
func createUser(t *testing.T, tx *sql.Tx, locale model.Locale) string {
	t.Helper()

	email := testutil.UniqueEmail("sign-in")
	userID := testutil.NewUserBuilder(t, tx).WithEmail(email).WithLocale(locale).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword(testPassword).Build()
	return email
}

// postSignIn はログインのフォームの送信を組み立てる。
func postSignIn(form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/sign_in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
}

// cookie は応答が設定したCookieをnameで探す。無ければnilを返す。
func cookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Success は、資格情報が一致すればセッションCookieを発行し、
// ユーザーの言語のフラッシュメッセージを付けて行き先へ303で送ることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		returnTo     string
		wantLocation string
	}{
		{name: "戻り先が無ければホームへ送る", returnTo: "", wantLocation: "/home"},
		{name: "安全な戻り先へ送る", returnTo: "/home?tab=1", wantLocation: "/home?tab=1"},
		{name: "別のオリジンを指す戻り先は捨ててホームへ送る", returnTo: "https://evil.example.com/", wantLocation: "/home"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			turnstileVerifier := &testutil.FakeTurnstileVerifier{Passed: true}
			handler, tx := newHandler(t, testConfig(), turnstileVerifier)
			// 日本語版の画面からログインした英語のユーザー。
			email := createUser(t, tx, model.LocaleEn)

			rec := httptest.NewRecorder()
			handler.Create(rec, postSignIn(url.Values{
				"email":                 {email},
				"password":              {testPassword},
				"return_to":             {tt.returnTo},
				"cf-turnstile-response": {"turnstile-token"},
			}))

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("ステータスコード = %d、期待値 = %d\n%s", rec.Code, http.StatusSeeOther, rec.Body.String())
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("Location = %q、期待値 = %q", got, tt.wantLocation)
			}
			if turnstileVerifier.Token != "turnstile-token" {
				t.Errorf("Turnstileへ渡したトークン = %q、期待値 = %q", turnstileVerifier.Token, "turnstile-token")
			}
			if c := cookie(rec, session.CookieName); c == nil || c.Value == "" {
				t.Error("セッションCookieが発行されていない")
			}
			if c := cookie(rec, session.FlashCookieName); c == nil || c.Value == "" {
				t.Fatal("フラッシュCookieが発行されていない")
			}
		})
	}
}

// TestCreate_TwoFactorAuthRequired は、二要素認証を有効にしたユーザーには、パスワードが合ってもセッションを発行せず、
// コードの入力を待つCookieを発行して、表示中の言語版のコードの入力画面へ戻り先を引き継いで送ることを検証する。
func TestCreate_TwoFactorAuthRequired(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t, testConfig(), &testutil.FakeTurnstileVerifier{Passed: true})
	email := testutil.UniqueEmail("sign-in-two-factor")
	userID := testutil.NewUserBuilder(t, tx).WithEmail(email).Build()
	testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword(testPassword).Build()
	testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).WithEnabledAt(time.Now()).Build()

	req := postSignIn(url.Values{"email": {email}, "password": {testPassword}, "return_to": {"/settings/invitation"}})
	req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangEn))
	rec := httptest.NewRecorder()
	handler.Create(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d\n%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if got, want := rec.Header().Get("Location"), "/en/sign_in/two_factor?return_to=%2Fsettings%2Finvitation"; got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if c := cookie(rec, session.CookieName); c != nil {
		t.Error("コードを入力する前にセッションCookieが発行された")
	}
	if c := cookie(rec, session.FlashCookieName); c != nil {
		t.Error("コードを入力する前にログインのフラッシュメッセージが出た")
	}
	pending := cookie(rec, session.TwoFactorPendingCookieName)
	if pending == nil {
		t.Fatal("コードの入力を待つCookieが発行されていない")
	}
	next := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor", nil)
	next.AddCookie(pending)
	if got, ok := session.NewContinuationManager(testContinuationKey).TwoFactorPendingUserID(next); !ok || got != userID {
		t.Errorf("Cookieのユーザー = (%v, %t)、期待値 = (%v, true)", got, ok, userID)
	}
}

// TestCreate_Rejected は、受け付けない送信をセッションを発行せずに再描画し、
// メールアドレスは戻してパスワードは戻さないことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		turnstile  *testutil.FakeTurnstileVerifier
		password   string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "パスワードが違う",
			turnstile:  &testutil.FakeTurnstileVerifier{Passed: true},
			password:   "wrong-password",
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "メールアドレスまたはパスワードが正しくありません",
		},
		{
			name:       "Turnstileの確認を通過しなかった",
			turnstile:  &testutil.FakeTurnstileVerifier{Passed: false},
			password:   testPassword,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "ロボットによる操作ではないことを確認できませんでした",
		},
		{
			name:       "Turnstileの確認でエラーが起きた",
			turnstile:  &testutil.FakeTurnstileVerifier{Err: errors.New("siteverifyに届かない")},
			password:   testPassword,
			wantStatus: http.StatusUnprocessableEntity,
			wantBody:   "ロボットによる操作ではないことを確認できませんでした",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t, testConfig(), tt.turnstile)
			email := createUser(t, tx, model.LocaleJa)

			rec := httptest.NewRecorder()
			handler.Create(rec, postSignIn(url.Values{"email": {email}, "password": {tt.password}}))

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("レスポンスボディに %q が含まれていない", tt.wantBody)
			}
			if !strings.Contains(body, `value="`+email+`"`) {
				t.Error("入力したメールアドレスがフォームへ戻っていない")
			}
			if strings.Contains(body, tt.password) {
				t.Error("入力したパスワードがレスポンスボディに含まれている")
			}
			if c := cookie(rec, session.CookieName); c != nil {
				t.Error("受け付けない送信でセッションCookieが発行された")
			}
		})
	}
}

// TestCreate_RateLimited は、同じメールアドレスへの試行が上限を超えると、
// 正しいパスワードでも照合せずに429とRetry-Afterで応えることを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t, testConfig(), &testutil.FakeTurnstileVerifier{Passed: true})
	email := createUser(t, tx, model.LocaleJa)

	// メールアドレスの上限 (10回) まで誤ったパスワードで試す。
	for range 10 {
		rec := httptest.NewRecorder()
		handler.Create(rec, postSignIn(url.Values{"email": {email}, "password": {"wrong-password"}}))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("上限内の試行のステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
		}
	}

	// 大文字小文字を変えても同じアカウントの試行として数える。
	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(url.Values{"email": {strings.ToUpper(email)}, "password": {testPassword}}))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter <= 0 || retryAfter > 15*60 {
		t.Errorf("Retry-After = %q、1〜900秒を期待", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "試行の回数が多すぎます") {
		t.Error("レスポンスボディにレート制限のメッセージが含まれていない")
	}
	if c := cookie(rec, session.CookieName); c != nil {
		t.Error("上限を超えた試行でセッションCookieが発行された")
	}
}

// TestCreate_ValidationErrors は、形式の誤りを入力欄に関連付けて示し、フォームの冒頭に要約することを検証する。
func TestCreate_ValidationErrors(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t, testConfig(), &testutil.FakeTurnstileVerifier{Passed: true})

	rec := httptest.NewRecorder()
	handler.Create(rec, postSignIn(url.Values{"email": {"not-an-email"}}))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"入力内容を確認してください",
		`aria-invalid="true" aria-describedby="email-error-0"`,
		`<p id="email-error-0" role="alert">メールアドレスの形式が正しくありません</p>`,
		`<p id="password-error-0" role="alert">入力してください</p>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestCreate_IPRateLimited は宛先を変えても同じIPの試行を数え、別IPには影響しないことを確かめる。
func TestCreate_IPRateLimited(t *testing.T) {
	t.Parallel()
	handler, _ := newHandler(t, testConfig(), &testutil.FakeTurnstileVerifier{Passed: true})
	for i := range 32 {
		req := postSignIn(url.Values{"email": {testutil.UniqueEmail("ip-limit")}, "password": {"incorrect-password"}})
		req.RemoteAddr = "203.0.113.10:12345"
		if i == 31 {
			req.RemoteAddr = "203.0.113.11:12345"
		}
		rec := httptest.NewRecorder()
		handler.Create(rec, req)
		wantStatus := http.StatusUnprocessableEntity
		if i == 30 {
			wantStatus = http.StatusTooManyRequests
		}
		if rec.Code != wantStatus {
			t.Fatalf("%d回目のステータス = %d、期待値 = %d", i+1, rec.Code, wantStatus)
		}
		if i == 30 {
			seconds, err := strconv.Atoi(rec.Header().Get("Retry-After"))
			if err != nil || seconds <= 0 || seconds > 900 {
				t.Errorf("Retry-After = %q、1〜900秒を期待", rec.Header().Get("Retry-After"))
			}
		}
		if c := cookie(rec, session.CookieName); c != nil {
			t.Error("拒否された試行でセッションCookieが発行された")
		}
	}
}
