package email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreate は、一致するコードで確認のCookieを確認済みのCookieへ置き換え、招待のCookieを発行し直して、
// 表示中の言語版のアカウントの作成へ送ることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		target       string
		locale       string
		wantLocation string
	}{
		{target: "/email_confirmation", locale: i18n.LangJa, wantLocation: "/account"},
		{target: "/en/email_confirmation", locale: i18n.LangEn, wantLocation: "/en/account"},
	}
	for _, tt := range tests {
		invitationID := testutil.NewInvitationBuilder(t, db).Build()
		confirmationID := testutil.NewEmailConfirmationBuilder(t, db).Build()

		rec := httptest.NewRecorder()
		newHandler(t).Create(rec, newRequest(http.MethodPost, tt.target, url.Values{"code": {"123456"}}, tt.locale, "192.0.2.61:1234", continuationCookies(invitationID, confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q\n%s", tt.target, rec.Code, rec.Header().Get("Location"), tt.wantLocation, rec.Body.String())
		}
		if cookie := responseCookie(rec, session.EmailConfirmationCookieName); cookie == nil || cookie.MaxAge >= 0 {
			t.Errorf("%s: 確認のCookie = %+v、削除を期待", tt.target, cookie)
		}
		assertInvitationCookie(t, rec, invitationID)

		confirmed := responseCookie(rec, session.ConfirmedEmailCookieName)
		if confirmed == nil {
			t.Fatalf("%s: 確認済みのCookieが書き込まれていない", tt.target)
		}
		req := httptest.NewRequest(http.MethodGet, tt.wantLocation, nil)
		req.AddCookie(confirmed)
		if got, ok := session.NewContinuationManager(continuationKey).ConfirmedEmailConfirmationID(req); !ok || got != confirmationID {
			t.Errorf("%s: 確認済みのCookieのID = (%v, %t)、期待値 = (%v, true)", tt.target, got, ok, confirmationID)
		}
	}
}

// TestCreate_Rejected は、照合できなかった送信を入力を戻して422で再描画し、確認済みのCookieを書き込まないことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		name         string
		builder      *testutil.EmailConfirmationBuilder
		code         string
		wantContains []string
	}{
		{
			name:         "一致しないコード",
			builder:      testutil.NewEmailConfirmationBuilder(t, db),
			code:         "000000",
			wantContains: []string{"確認コードが正しくありません", `value="000000"`, `aria-invalid="true"`},
		},
		{
			name:         "形式の誤り",
			builder:      testutil.NewEmailConfirmationBuilder(t, db),
			code:         "abc",
			wantContains: []string{"6桁の数字で入力してください", `value="abc"`, `aria-invalid="true"`},
		},
		{
			name:         "期限切れ",
			builder:      testutil.NewEmailConfirmationBuilder(t, db).WithExpiresAt(time.Now().Add(-time.Minute)),
			code:         "123456",
			wantContains: []string{"この確認コードは使えなくなりました", "コードを再送する"},
		},
	}
	for _, tt := range tests {
		invitationID := testutil.NewInvitationBuilder(t, db).Build()

		rec := httptest.NewRecorder()
		newHandler(t).Create(rec, newRequest(http.MethodPost, "/email_confirmation", url.Values{"code": {tt.code}}, i18n.LangJa, "192.0.2.62:1234", continuationCookies(invitationID, tt.builder.Build())))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusUnprocessableEntity)
		}
		for _, want := range tt.wantContains {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("%s: 応答に %q が無い", tt.name, want)
			}
		}
		if responseCookie(rec, session.ConfirmedEmailCookieName) != nil {
			t.Errorf("%s: 照合できなかった送信で確認済みのCookieを書き込んだ", tt.name)
		}
	}
}

// TestCreate_RedirectsWithoutUsableContinuation は、確認または使える招待を持たない送信を照合せずに登録の画面へ戻すことを検証する。
func TestCreate_RedirectsWithoutUsableContinuation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		name           string
		invitationID   model.InvitationID
		confirmationID model.EmailConfirmationID
	}{
		{name: "確認無し", invitationID: testutil.NewInvitationBuilder(t, db).Build()},
		{name: "招待無し", confirmationID: testutil.NewEmailConfirmationBuilder(t, db).Build()},
		{
			name:           "取り消し済みの招待",
			invitationID:   testutil.NewInvitationBuilder(t, db).WithRevokedAt(time.Now()).Build(),
			confirmationID: testutil.NewEmailConfirmationBuilder(t, db).Build(),
		},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		newHandler(t).Create(rec, newRequest(http.MethodPost, "/email_confirmation", url.Values{"code": {"123456"}}, i18n.LangJa, "192.0.2.63:1234", continuationCookies(tt.invitationID, tt.confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_up" {
			t.Errorf("%s: 応答 = %d %q、303 /sign_up を期待", tt.name, rec.Code, rec.Header().Get("Location"))
		}
		if responseCookie(rec, session.ConfirmedEmailCookieName) != nil {
			t.Errorf("%s: 確認済みのCookieを書き込んだ", tt.name)
		}
	}
}

// TestCreate_RateLimited は、同じIPアドレスからの照合が上限を超えると429とRetry-Afterで拒むことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	handler := newHandler(t)
	cookies := continuationCookies(testutil.NewInvitationBuilder(t, db).Build(), testutil.NewEmailConfirmationBuilder(t, db).Build())
	send := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/email_confirmation", url.Values{"code": {"000000"}}, i18n.LangJa, "192.0.2.64:1234", cookies))
		return rec
	}

	for i := range 30 {
		if rec := send(); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusUnprocessableEntity)
		}
	}

	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が付いていない")
	}
}
