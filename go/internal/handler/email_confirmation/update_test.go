package email_confirmation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// countConfirmationJobs は指定したメールアドレス宛てに投入された確認コードのメールのジョブの件数を返す。
func countConfirmationJobs(t *testing.T, email string) int {
	t.Helper()

	var count int
	err := testutil.GetTestDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM river_job WHERE kind = 'send_email_confirmation' AND args->>'email' = $1", email).Scan(&count)
	if err != nil {
		t.Fatalf("ジョブの件数の取得のエラー = %v", err)
	}

	return count
}

// TestUpdate は、誤入力の上限に達した確認や期限切れの確認からも同じメールアドレスへ新しいコードを送り、
// 新しい確認のCookie・発行し直した招待のCookie・フラッシュメッセージを添えて表示中の言語版の入力画面へ戻すことを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		target       string
		locale       string
		wantLocation string
		expired      bool
	}{
		{target: "/email_confirmation", locale: i18n.LangJa, wantLocation: "/email_confirmation"},
		{target: "/en/email_confirmation", locale: i18n.LangEn, wantLocation: "/en/email_confirmation"},
		{target: "/email_confirmation", locale: i18n.LangJa, wantLocation: "/email_confirmation", expired: true},
	}
	for _, tt := range tests {
		email := testutil.UniqueEmail("email-confirmation-resend")
		invitationID := testutil.NewInvitationBuilder(t, db).Build()
		builder := testutil.NewEmailConfirmationBuilder(t, db).WithEmail(email)
		if tt.expired {
			builder.WithExpiresAt(time.Now().Add(-time.Minute))
		} else {
			builder.WithFailedAttemptsCount(model.EmailConfirmationMaxFailedAttempts)
		}
		confirmationID := builder.Build()

		rec := httptest.NewRecorder()
		newHandler(t).Update(rec, newRequest(http.MethodPatch, tt.target, url.Values{}, tt.locale, "192.0.2.65:1234", continuationCookies(invitationID, confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q\n%s", tt.target, rec.Code, rec.Header().Get("Location"), tt.wantLocation, rec.Body.String())
		}

		cookie := responseCookie(rec, session.EmailConfirmationCookieName)
		if cookie == nil {
			t.Fatalf("%s: 確認のCookieが書き込まれていない", tt.target)
		}
		req := httptest.NewRequest(http.MethodGet, tt.wantLocation, nil)
		req.AddCookie(cookie)
		if got, ok := session.NewContinuationManager(continuationKey).EmailConfirmationID(req); !ok || got == confirmationID {
			t.Errorf("%s: 確認のCookieのID = (%v, %t)、元とは別の新しい確認を期待", tt.target, got, ok)
		}

		assertInvitationCookie(t, rec, invitationID)

		if responseCookie(rec, session.FlashCookieName) == nil {
			t.Errorf("%s: フラッシュメッセージのCookieが書き込まれていない", tt.target)
		}
		if got := countConfirmationJobs(t, email); got != 1 {
			t.Errorf("%s: 確認コードのメールのジョブの件数 = %d、期待値 = 1", tt.target, got)
		}
	}
}

// TestUpdate_RedirectsToSignUp は、再送できる確認や使える招待を持たない送信を、メールを送らずに登録の画面へ戻すことを検証する。
func TestUpdate_RedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	email := testutil.UniqueEmail("email-confirmation-resend-rejected")
	tests := []struct {
		name                   string
		invitationID           model.InvitationID
		confirmationID         model.EmailConfirmationID
		wantConfirmationDelete bool
	}{
		{name: "確認無し", invitationID: testutil.NewInvitationBuilder(t, db).Build()},
		{
			name:           "取り消し済みの招待",
			invitationID:   testutil.NewInvitationBuilder(t, db).WithRevokedAt(time.Now()).Build(),
			confirmationID: testutil.NewEmailConfirmationBuilder(t, db).WithEmail(email).Build(),
		},
		{
			name:                   "確認済み",
			invitationID:           testutil.NewInvitationBuilder(t, db).Build(),
			confirmationID:         testutil.NewEmailConfirmationBuilder(t, db).WithEmail(email).WithConfirmedAt(time.Now()).Build(),
			wantConfirmationDelete: true,
		},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		newHandler(t).Update(rec, newRequest(http.MethodPatch, "/email_confirmation", url.Values{}, i18n.LangJa, "192.0.2.66:1234", continuationCookies(tt.invitationID, tt.confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_up" {
			t.Errorf("%s: 応答 = %d %q、303 /sign_up を期待", tt.name, rec.Code, rec.Header().Get("Location"))
		}
		if tt.wantConfirmationDelete {
			if cookie := responseCookie(rec, session.EmailConfirmationCookieName); cookie == nil || cookie.MaxAge >= 0 {
				t.Errorf("%s: 確認のCookie = %+v、削除を期待", tt.name, cookie)
			}
		}
	}
	if got := countConfirmationJobs(t, email); got != 0 {
		t.Errorf("確認コードのメールのジョブの件数 = %d、期待値 = 0", got)
	}
}

// TestUpdate_RateLimited は、同じメールアドレスへの再送が上限を超えると429とRetry-Afterで拒むことを検証する。
func TestUpdate_RateLimited(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	handler := newHandler(t)
	email := testutil.UniqueEmail("email-confirmation-resend-rate-limited")
	cookies := continuationCookies(testutil.NewInvitationBuilder(t, db).Build(), testutil.NewEmailConfirmationBuilder(t, db).WithEmail(email).Build())
	send := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.Update(rec, newRequest(http.MethodPatch, "/email_confirmation", url.Values{}, i18n.LangJa, "192.0.2.67:1234", cookies))
		return rec
	}

	for i := range 5 {
		if rec := send(); rec.Code != http.StatusSeeOther {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	rec := send()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が付いていない")
	}
	if got := countConfirmationJobs(t, email); got != 5 {
		t.Errorf("確認コードのメールのジョブの件数 = %d、期待値 = 5", got)
	}
}
