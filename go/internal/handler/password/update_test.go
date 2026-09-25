package password_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// assertPassword はトークンの持ち主の保存したパスワードがpasswordと一致することを確かめる。
func assertPassword(t *testing.T, email, password string) {
	t.Helper()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user, err := repository.NewUserRepository(db).FindByEmail(ctx, email)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%+v, %v)、ユーザーを期待", user, err)
	}
	stored, err := repository.NewUserPasswordRepository(db).FindByUserID(ctx, user.ID)
	if err != nil || stored == nil {
		t.Fatalf("パスワードの取得 = (%+v, %v)、パスワードを期待", stored, err)
	}
	if err := auth.CheckPassword(stored.PasswordDigest, password); err != nil {
		t.Errorf("保存したパスワードが %q と一致しない: %v", password, err)
	}
}

// TestUpdate は、新しいパスワードを設定し、トークンのCookieを消して、表示中の言語版のログイン画面へ送ることを検証する。
func TestUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target       string
		locale       string
		wantLocation string
	}{
		{target: "/password", locale: i18n.LangJa, wantLocation: "/sign_in"},
		{target: "/en/password", locale: i18n.LangEn, wantLocation: "/en/sign_in"},
	}
	for _, tt := range tests {
		_, id, email := resetToken(t)

		rec := httptest.NewRecorder()
		newHandler().Update(rec, newRequest(http.MethodPatch, tt.target, url.Values{"password": {"new-password1234"}}, tt.locale, tokenCookies(id)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Fatalf("%s: 応答 = %d %q、期待値 = 303 %q\n%s", tt.target, rec.Code, rec.Header().Get("Location"), tt.wantLocation, rec.Body.String())
		}
		if cookie := responseCookie(rec, session.PasswordResetCookieName); cookie == nil || cookie.MaxAge >= 0 {
			t.Errorf("%s: %s = %+v、削除を期待", tt.target, session.PasswordResetCookieName, cookie)
		}
		if responseCookie(rec, session.FlashCookieName) == nil {
			t.Errorf("%s: フラッシュメッセージが書き込まれていない", tt.target)
		}
		if responseCookie(rec, session.CookieName) != nil {
			t.Errorf("%s: セッションCookieが書き込まれている。ログインさせないことを期待", tt.target)
		}
		assertPassword(t, email, "new-password1234")
	}
}

// TestUpdate_Rejected は、受け付けなかった送信を422で再描画し、トークンのCookieを残して送り直せるようにすることを検証する。
func TestUpdate_Rejected(t *testing.T) {
	t.Parallel()

	_, id, email := resetToken(t)

	rec := httptest.NewRecorder()
	newHandler().Update(rec, newRequest(http.MethodPatch, "/password", url.Values{"password": {"short"}}, i18n.LangJa, tokenCookies(id)))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータス = %d、期待値 = 422", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"8文字以上で入力してください", `value="` + email + `"`, `aria-invalid="true"`} {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
	if responseCookie(rec, session.PasswordResetCookieName) != nil {
		t.Error("トークンのCookieを書き換えている。残すことを期待")
	}
	assertPassword(t, email, testutil.DefaultBuilderPassword)
}

// TestUpdate_UnusableToken は、使用済みのトークンでは、パスワードを変えずに案内を404で示すことを検証する。
func TestUpdate_UnusableToken(t *testing.T) {
	t.Parallel()

	_, id, email := resetToken(t)
	cookies := tokenCookies(id)
	handler := newHandler()

	rec := httptest.NewRecorder()
	handler.Update(rec, newRequest(http.MethodPatch, "/password", url.Values{"password": {"new-password1234"}}, i18n.LangJa, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("1回目のステータス = %d、期待値 = 303", rec.Code)
	}

	// 消える前のCookieを使い回して、同じトークンで2度目の設定を試みる。
	rec = httptest.NewRecorder()
	handler.Update(rec, newRequest(http.MethodPatch, "/password", url.Values{"password": {"another-password"}}, i18n.LangJa, cookies))

	assertUnusable(t, "使用済みのトークン", rec)
	assertPassword(t, email, "new-password1234")
}

// TestUpdate_WithoutCookie は、Cookieが無いときにパスワードを変えずに案内を404で示すことを検証する。
func TestUpdate_WithoutCookie(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	newHandler().Update(rec, newRequest(http.MethodPatch, "/password", url.Values{"password": {"new-password1234"}}, i18n.LangJa, nil))

	assertUnusable(t, "Cookie無し", rec)
}
