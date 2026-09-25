package sign_in_two_factor_test

import (
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

// postCreate はコードの送信を、パスワードを確かめたCookieを載せて組み立てる。cookie がnilなら載せない。
func postCreate(form url.Values, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/sign_in/two_factor", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa))
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
// ユーザーの言語のフラッシュメッセージを付けて行き先へ303で送ることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		returnTo     string
		wantLocation string
	}{
		{name: "戻り先が無ければホームへ送る", returnTo: "", wantLocation: "/home"},
		{name: "安全な戻り先へ送る", returnTo: "/settings/invitation", wantLocation: "/settings/invitation"},
		{name: "別のオリジンを指す戻り先は捨ててホームへ送る", returnTo: "https://evil.example.com/", wantLocation: "/home"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			userID, secret := createTwoFactorUser(t, tx, model.LocaleEn)

			rec := httptest.NewRecorder()
			handler.Create(rec, postCreate(url.Values{"code": {currentCode(t, secret)}, "return_to": {tt.returnTo}}, pendingCookie(userID)))

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

	handler, tx := newHandler(t)
	userID, secret := createTwoFactorUser(t, tx, model.LocaleJa)
	code := currentCode(t, secret)
	cookie := pendingCookie(userID)

	// 1回目で使ったコードを、同じタイムステップのうちに送り直す。
	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(url.Values{"code": {code}}, cookie))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("1回目のステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	wrongCode := "000000"
	if wrongCode == code {
		wrongCode = "111111"
	}
	tests := []struct {
		name    string
		code    string
		wantMsg string
	}{
		{name: "使用済みのコード", code: code, wantMsg: "コードが正しくありません"},
		{name: "誤ったコード", code: wrongCode, wantMsg: "コードが正しくありません"},
		{name: "形式の誤り", code: "12a456", wantMsg: "6桁の数字で入力してください"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		handler.Create(rec, postCreate(url.Values{"code": {tt.code}, "return_to": {"/home"}}, cookie))

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

// TestCreate_WithoutPendingCookie は、パスワードを確かめたCookieが無ければ、コードを照合せずログイン画面へ送ることを検証する。
func TestCreate_WithoutPendingCookie(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	_, secret := createTwoFactorUser(t, tx, model.LocaleJa)

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(url.Values{"code": {currentCode(t, secret)}, "return_to": {"/home"}}, nil))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_in?return_to=%2Fhome" {
		t.Errorf("応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/sign_in?return_to=%2Fhome")
	}
	if c := findCookie(rec, session.CookieName); c != nil {
		t.Error("Cookieの無い送信でセッションCookieが発行された")
	}
}

// TestCreate_TwoFactorAuthGone は、パスワードを確かめた後に二要素認証が有効でなくなったときに、
// コードの入力を待つCookieを消してログイン画面へ送り、パスワードの確認からやり直させることを検証する。
func TestCreate_TwoFactorAuthGone(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	userID := testutil.NewUserBuilder(t, tx).Build()

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(url.Values{"code": {"123456"}}, pendingCookie(userID)))

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
// 正しいコードを照合せずに429とRetry-Afterで応え、次に試せるまでの分数を示すことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	userID, secret := createTwoFactorUser(t, tx, model.LocaleJa)
	cookie := pendingCookie(userID)

	// ユーザーの上限 (5回) まで、IPアドレスを変えながら誤ったコードで試す。
	for i := range 5 {
		req := postCreate(url.Values{"code": {"000000"}}, cookie)
		req.RemoteAddr = "203.0.113." + strconv.Itoa(20+i) + ":12345"
		rec := httptest.NewRecorder()
		handler.Create(rec, req)
		if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusSeeOther {
			t.Fatalf("上限内の試行のステータスコード = %d、422か (偶然一致した) 303を期待", rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postCreate(url.Values{"code": {currentCode(t, secret)}}, cookie))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter <= 0 || retryAfter > int((15*time.Minute).Seconds()) {
		t.Errorf("Retry-After = %q、1〜900秒を期待", rec.Header().Get("Retry-After"))
	}
	body := rec.Body.String()
	wantMinutes := (retryAfter + 59) / 60
	if want := "試行の回数が上限に達しました。あと" + strconv.Itoa(wantMinutes) + "分で試せます"; !strings.Contains(body, want) {
		t.Errorf("レスポンスボディに %q が含まれていない", want)
	}
	if c := findCookie(rec, session.CookieName); c != nil {
		t.Error("上限を超えた試行でセッションCookieが発行された")
	}
}
