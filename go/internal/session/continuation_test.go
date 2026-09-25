package session_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
)

const testContinuationKey = "test-continuation-token-key-0123456789"

// TestContinuationManager_InvitationID は、書き込んだ招待のIDをCookieから読み戻せ、
// Cookieが __Host- 接頭辞の条件とHttpOnlyを満たすことを検証する。
func TestContinuationManager_InvitationID(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)
	id := model.InvitationID(uuid.New())

	rec := httptest.NewRecorder()
	m.SetInvitationID(rec, id)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Cookieの件数 = %d、期待値 = 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != session.InvitationCookieName || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" ||
		cookie.Domain != "" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge <= 0 {
		t.Errorf("Cookie = %+v、__Host- の条件 (Secure・Path=/・Domain無し) とHttpOnly・SameSite=Lax・正のMaxAgeを期待", cookie)
	}

	req := httptest.NewRequest(http.MethodGet, "/sign_up", nil)
	req.AddCookie(cookie)
	got, ok := m.InvitationID(req)
	if !ok || got != id {
		t.Errorf("InvitationID() = (%v, %t)、期待値 = (%v, true)", got, ok, id)
	}
}

// TestContinuationManager_InvitationID_Invalid は、Cookieが無い / 偽造されているときに招待を読み取らないことを検証する。
func TestContinuationManager_InvitationID_Invalid(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)

	for name, value := range map[string]string{
		"Cookie無し": "",
		"署名の無い値":   "v1.invitation." + uuid.NewString() + ".9999999999",
		"署名を偽った値":  "v1.invitation." + uuid.NewString() + ".9999999999.forged",
	} {
		req := httptest.NewRequest(http.MethodGet, "/sign_up", nil)
		if value != "" {
			req.AddCookie(&http.Cookie{Name: session.InvitationCookieName, Value: value})
		}

		if _, ok := m.InvitationID(req); ok {
			t.Errorf("%s: InvitationID()の結果 = true、期待値 = false", name)
		}
	}
}

// TestContinuationManager_DeleteInvitationID は、同じ属性でMaxAgeを負にしたCookieを送り、ブラウザに削除させることを検証する。
func TestContinuationManager_DeleteInvitationID(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	session.NewContinuationManager(testContinuationKey).DeleteInvitationID(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != session.InvitationCookieName || cookies[0].MaxAge >= 0 || !cookies[0].Secure {
		t.Errorf("Cookie = %+v、Secureで負のMaxAgeの %s を期待", cookies, session.InvitationCookieName)
	}
}

// TestNewContinuationManager_ShortKey は、最小の長さに満たない鍵では生成を拒むことを検証する。
func TestNewContinuationManager_ShortKey(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"", strings.Repeat("k", config.ContinuationTokenMinimumKeyLength-1)} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%dバイトの鍵でpanicしなかった", len(key))
				}
			}()
			session.NewContinuationManager(key)
		}()
	}
}

// TestContinuationManager_EmailConfirmationID は、確認のIDを読み戻せ、Cookieがコードの期限切れ後も再送に使える寿命を持ち、
// 招待のCookieを確認のCookieとして使い回せないことを検証する。
func TestContinuationManager_EmailConfirmationID(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)
	id := model.EmailConfirmationID(uuid.New())

	rec := httptest.NewRecorder()
	m.SetEmailConfirmationID(rec, id)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != session.EmailConfirmationCookieName || cookies[0].MaxAge != int(time.Hour.Seconds()) {
		t.Fatalf("Cookie = %+v、MaxAgeが再送できる期間の %s を期待", cookies, session.EmailConfirmationCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/email_confirmation", nil)
	req.AddCookie(cookies[0])
	if got, ok := m.EmailConfirmationID(req); !ok || got != id {
		t.Errorf("EmailConfirmationID() = (%v, %t)、期待値 = (%v, true)", got, ok, id)
	}

	// 招待のCookieの値を確認のCookieの名前で送っても、用途が違うため読み取らない。
	invitationRec := httptest.NewRecorder()
	m.SetInvitationID(invitationRec, model.InvitationID(uuid.New()))
	req = httptest.NewRequest(http.MethodGet, "/email_confirmation", nil)
	req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: invitationRec.Result().Cookies()[0].Value})
	if _, ok := m.EmailConfirmationID(req); ok {
		t.Error("招待のCookieの値を確認のIDとして読み取った")
	}

	deleteRec := httptest.NewRecorder()
	m.DeleteEmailConfirmationID(deleteRec)
	if deleted := deleteRec.Result().Cookies(); len(deleted) != 1 || deleted[0].MaxAge >= 0 {
		t.Errorf("削除のCookie = %+v、負のMaxAgeを期待", deleted)
	}
}

// TestContinuationManager_ConfirmedEmailConfirmationID は、確認済みの確認のIDを読み戻せ、
// 確認コードを入力する前のCookieを確認済みのCookieとして使い回せないことを検証する。
func TestContinuationManager_ConfirmedEmailConfirmationID(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)
	id := model.EmailConfirmationID(uuid.New())

	rec := httptest.NewRecorder()
	m.SetConfirmedEmailConfirmationID(rec, id)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != session.ConfirmedEmailCookieName || cookies[0].MaxAge <= int(model.EmailConfirmationLifetime.Seconds()) {
		t.Fatalf("Cookie = %+v、MaxAgeが確認コードの有効期間より長い %s を期待", cookies, session.ConfirmedEmailCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(cookies[0])
	if got, ok := m.ConfirmedEmailConfirmationID(req); !ok || got != id {
		t.Errorf("ConfirmedEmailConfirmationID() = (%v, %t)、期待値 = (%v, true)", got, ok, id)
	}

	// 確認コードを入力する前のCookieの値を送っても、用途が違うため読み取らない。
	unconfirmedRec := httptest.NewRecorder()
	m.SetEmailConfirmationID(unconfirmedRec, id)
	req = httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(&http.Cookie{Name: session.ConfirmedEmailCookieName, Value: unconfirmedRec.Result().Cookies()[0].Value})
	if _, ok := m.ConfirmedEmailConfirmationID(req); ok {
		t.Error("確認コードを入力する前のCookieの値を確認済みのIDとして読み取った")
	}
}

// TestContinuationManager_PasswordResetTokenID は、トークンのIDを読み戻せ、Cookieがトークンと同じ寿命を持ち、
// 招待のCookieをパスワードリセットのCookieとして使い回せないことを検証する。
func TestContinuationManager_PasswordResetTokenID(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)
	id := model.PasswordResetTokenID(uuid.New())

	rec := httptest.NewRecorder()
	m.SetPasswordResetTokenID(rec, id)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != session.PasswordResetCookieName || cookies[0].MaxAge != int(model.PasswordResetTokenLifetime.Seconds()) {
		t.Fatalf("Cookie = %+v、MaxAgeがトークンの有効期間の %s を期待", cookies, session.PasswordResetCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/password", nil)
	req.AddCookie(cookies[0])
	if got, ok := m.PasswordResetTokenID(req); !ok || got != id {
		t.Errorf("PasswordResetTokenID() = (%v, %t)、期待値 = (%v, true)", got, ok, id)
	}

	// 招待のCookieの値をパスワードリセットのCookieの名前で送っても、用途が違うため読み取らない。
	invitationRec := httptest.NewRecorder()
	m.SetInvitationID(invitationRec, model.InvitationID(uuid.New()))
	req = httptest.NewRequest(http.MethodGet, "/password", nil)
	req.AddCookie(&http.Cookie{Name: session.PasswordResetCookieName, Value: invitationRec.Result().Cookies()[0].Value})
	if _, ok := m.PasswordResetTokenID(req); ok {
		t.Error("招待のCookieの値をパスワードリセットのトークンのIDとして読み取った")
	}

	deleteRec := httptest.NewRecorder()
	m.DeletePasswordResetTokenID(deleteRec)
	if deleted := deleteRec.Result().Cookies(); len(deleted) != 1 || deleted[0].Name != session.PasswordResetCookieName || deleted[0].MaxAge >= 0 {
		t.Errorf("削除のCookie = %+v、負のMaxAgeを期待", deleted)
	}
}

// TestContinuationManager_TwoFactorPendingUserID は、ユーザーのIDを読み戻せ、Cookieが10分で切れ、
// 別の用途のCookieをコードの入力を待つユーザーとして使い回せないことを検証する。
func TestContinuationManager_TwoFactorPendingUserID(t *testing.T) {
	t.Parallel()

	m := session.NewContinuationManager(testContinuationKey)
	id := model.UserID(uuid.New())

	rec := httptest.NewRecorder()
	m.SetTwoFactorPendingUserID(rec, id)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != session.TwoFactorPendingCookieName || cookies[0].MaxAge != int((10*time.Minute).Seconds()) {
		t.Fatalf("Cookie = %+v、MaxAgeが10分の %s を期待", cookies, session.TwoFactorPendingCookieName)
	}

	req := httptest.NewRequest(http.MethodGet, "/sign_in/two_factor", nil)
	req.AddCookie(cookies[0])
	if got, ok := m.TwoFactorPendingUserID(req); !ok || got != id {
		t.Errorf("TwoFactorPendingUserID() = (%v, %t)、期待値 = (%v, true)", got, ok, id)
	}

	// パスワードリセットのCookieの値を送っても、用途が違うため読み取らない。
	passwordResetRec := httptest.NewRecorder()
	m.SetPasswordResetTokenID(passwordResetRec, model.PasswordResetTokenID(uuid.New()))
	req = httptest.NewRequest(http.MethodGet, "/sign_in/two_factor", nil)
	req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: passwordResetRec.Result().Cookies()[0].Value})
	if _, ok := m.TwoFactorPendingUserID(req); ok {
		t.Error("パスワードリセットのCookieの値をコードの入力を待つユーザーのIDとして読み取った")
	}

	deleteRec := httptest.NewRecorder()
	m.DeleteTwoFactorPendingUserID(deleteRec)
	if deleted := deleteRec.Result().Cookies(); len(deleted) != 1 || deleted[0].Name != session.TwoFactorPendingCookieName || deleted[0].MaxAge >= 0 {
		t.Errorf("削除のCookie = %+v、負のMaxAgeを期待", deleted)
	}
}
