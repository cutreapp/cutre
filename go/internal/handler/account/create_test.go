package account_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreate は、アカウントを表示中の言語で作ってログインさせ、登録の途中のCookieを消してホームへ送ることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		target     string
		locale     string
		wantLocale model.Locale
	}{
		{target: "/account", locale: i18n.LangJa, wantLocale: model.LocaleJa},
		{target: "/en/account", locale: i18n.LangEn, wantLocale: model.LocaleEn},
	}
	for _, tt := range tests {
		confirmationID, email := confirmedEmail(t)
		atname := testutil.UniqueAtname()

		rec := httptest.NewRecorder()
		newHandler().Create(rec, newRequest(http.MethodPost, tt.target, url.Values{"atname": {atname}, "password": {"password1234"}}, tt.locale, continuationCookies(testutil.NewInvitationBuilder(t, db).Build(), confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/home" {
			t.Fatalf("%s: 応答 = %d %q、303 /home を期待\n%s", tt.target, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
		if cookie := responseCookie(rec, session.CookieName); cookie == nil || cookie.Value == "" {
			t.Errorf("%s: セッションCookieが書き込まれていない", tt.target)
		}
		if responseCookie(rec, session.FlashCookieName) == nil {
			t.Errorf("%s: フラッシュメッセージが書き込まれていない", tt.target)
		}
		for _, name := range []string{session.InvitationCookieName, session.ConfirmedEmailCookieName} {
			if cookie := responseCookie(rec, name); cookie == nil || cookie.MaxAge >= 0 {
				t.Errorf("%s: %s = %+v、削除を期待", tt.target, name, cookie)
			}
		}

		user, err := repository.NewUserRepository(db).FindByEmail(context.Background(), email)
		if err != nil || user == nil {
			t.Fatalf("%s: ユーザー = (%+v, %v)、作成を期待", tt.target, user, err)
		}
		if user.Atname != atname || user.Locale != tt.wantLocale {
			t.Errorf("%s: ユーザー = %+v、アットネーム %q・ロケール %q を期待", tt.target, user, atname, tt.wantLocale)
		}
	}
}

// TestCreate_Rejected は、受け付けなかった送信を422で再描画し、アットネームとメールアドレスは戻してパスワードは戻さないことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	confirmationID, email := confirmedEmail(t)
	cookies := continuationCookies(testutil.NewInvitationBuilder(t, testutil.GetTestDB()).Build(), confirmationID)

	rec := httptest.NewRecorder()
	newHandler().Create(rec, newRequest(http.MethodPost, "/account", url.Values{"atname": {"cutre-user"}, "password": {"short"}}, i18n.LangJa, cookies))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータス = %d、期待値 = 422", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"半角英数字とアンダースコア (_) だけで入力してください",
		"8文字以上で入力してください",
		`value="cutre-user"`,
		`value="` + email + `"`,
		`aria-invalid="true"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("応答に %q が無い", want)
		}
	}
	if strings.Contains(body, `value="short"`) {
		t.Error("パスワードを入力欄へ戻した")
	}
	if responseCookie(rec, session.CookieName) != nil {
		t.Error("受け付けなかった送信でセッションCookieを書き込んだ")
	}
}

// TestCreate_InvitationUnusable は、招待が使えなくなっているとき、アカウントを作らずにその理由を403で示すことを検証する。
func TestCreate_InvitationUnusable(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	confirmationID, email := confirmedEmail(t)
	invitationID := testutil.NewInvitationBuilder(t, db).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	rec := httptest.NewRecorder()
	newHandler().Create(rec, newRequest(http.MethodPost, "/account", url.Values{"atname": {testutil.UniqueAtname()}, "password": {"password1234"}}, i18n.LangJa, continuationCookies(invitationID, confirmationID)))

	assertInvitationUnusable(t, rec)
	if user, err := repository.NewUserRepository(db).FindByEmail(context.Background(), email); err != nil || user != nil {
		t.Errorf("ユーザー = (%+v, %v)、作成しないことを期待", user, err)
	}
}

// TestCreate_RedirectsWithoutConfirmation は、確認済みの確認を持たない送信を、アカウントを作らずに登録の画面へ戻すことを検証する。
func TestCreate_RedirectsWithoutConfirmation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	atname := testutil.UniqueAtname()
	cookies := continuationCookies(testutil.NewInvitationBuilder(t, db).Build(), testutil.NewEmailConfirmationBuilder(t, db).Build())

	rec := httptest.NewRecorder()
	newHandler().Create(rec, newRequest(http.MethodPost, "/account", url.Values{"atname": {atname}, "password": {"password1234"}}, i18n.LangJa, cookies))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_up" {
		t.Errorf("応答 = %d %q、303 /sign_up を期待", rec.Code, rec.Header().Get("Location"))
	}
	if user, err := repository.NewUserRepository(db).FindByAtname(context.Background(), atname); err != nil || user != nil {
		t.Errorf("ユーザー = (%+v, %v)、作成しないことを期待", user, err)
	}
}
