package sign_up_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreate は、登録済みかどうかによらず同じ応答 (確認のCookie・発行し直した招待のCookieと、表示中の言語版の確認コードの入力画面へのリダイレクト) を返すことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	db := testutil.GetTestDB()

	registered := testutil.UniqueEmail("sign-up-handler-registered")
	testutil.NewUserBuilder(t, db).WithEmail(registered).Build()

	tests := []struct {
		name         string
		target       string
		locale       string
		email        string
		wantLocation string
	}{
		{name: "未登録のアドレス", target: "/sign_up", locale: i18n.LangJa, email: testutil.UniqueEmail("sign-up-handler-new"), wantLocation: "/email_confirmation"},
		{name: "登録済みのアドレス", target: "/sign_up", locale: i18n.LangJa, email: registered, wantLocation: "/email_confirmation"},
		{name: "英語版", target: "/en/sign_up", locale: i18n.LangEn, email: testutil.UniqueEmail("sign-up-handler-en"), wantLocation: "/en/email_confirmation"},
	}

	for _, tt := range tests {
		invitationID := testutil.NewInvitationBuilder(t, db).Build()
		cookieRec := httptest.NewRecorder()
		session.NewContinuationManager(testContinuationKey).SetInvitationID(cookieRec, invitationID)
		cookie := cookieRec.Result().Cookies()[0]
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, tt.target, url.Values{"email": {tt.email}}, tt.locale, "192.0.2.40:1234", cookie))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q\n%s", tt.name, rec.Code, rec.Header().Get("Location"), tt.wantLocation, rec.Body.String())
		}
		assertEmailConfirmationCookie(t, rec)
		assertInvitationCookie(t, rec, invitationID)
	}
}

// TestCreate_Rejected は、招待が無い・Bot対策を通らない・形式の誤りの送信を、確認を作らずに拒むことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	email := testutil.UniqueEmail("sign-up-handler-rejected")

	tests := []struct {
		name         string
		turnstile    *testutil.FakeTurnstileVerifier
		withCookie   bool
		email        string
		wantStatus   int
		wantContains string
	}{
		{name: "招待を持たない", turnstile: &testutil.FakeTurnstileVerifier{Passed: true}, email: email, wantStatus: http.StatusForbidden, wantContains: "Cutreは招待制です"},
		{name: "Bot対策を通らない", turnstile: &testutil.FakeTurnstileVerifier{Passed: false}, withCookie: true, email: email, wantStatus: http.StatusUnprocessableEntity, wantContains: `value="` + email + `"`},
		{name: "Bot対策の検証に失敗する", turnstile: &testutil.FakeTurnstileVerifier{Err: errors.New("siteverifyの障害")}, withCookie: true, email: email, wantStatus: http.StatusUnprocessableEntity},
		{name: "アドレスとして読めない", turnstile: &testutil.FakeTurnstileVerifier{Passed: true}, withCookie: true, email: "not-an-email", wantStatus: http.StatusUnprocessableEntity, wantContains: `aria-invalid="true"`},
	}

	for _, tt := range tests {
		handler := newHandler(t, tt.turnstile)
		cookies := []*http.Cookie{}
		if tt.withCookie {
			cookies = append(cookies, invitationCookie(t, testutil.NewInvitationBuilder(t, db)))
		}

		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/sign_up", url.Values{"email": {tt.email}}, i18n.LangJa, "192.0.2.41:1234", cookies...))

		if rec.Code != tt.wantStatus {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, tt.wantStatus)
		}
		if tt.wantContains != "" && !strings.Contains(rec.Body.String(), tt.wantContains) {
			t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, tt.wantContains)
		}
		for _, c := range rec.Result().Cookies() {
			if c.Name == "__Host-cutre_email_confirmation" {
				t.Errorf("%s: 拒んだ送信で確認のCookieを書き込んだ", tt.name)
			}
		}
	}
}

// TestCreate_RateLimited は、同じメールアドレスへの送信が上限を超えると429とRetry-Afterで拒むことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	db := testutil.GetTestDB()
	email := testutil.UniqueEmail("sign-up-handler-rate-limited")

	for i := range 5 {
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/sign_up", url.Values{"email": {email}}, i18n.LangJa, "192.0.2.42:1234", invitationCookie(t, testutil.NewInvitationBuilder(t, db))))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, newRequest(http.MethodPost, "/sign_up", url.Values{"email": {email}}, i18n.LangJa, "192.0.2.42:1234", invitationCookie(t, testutil.NewInvitationBuilder(t, db))))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が付いていない")
	}
}
