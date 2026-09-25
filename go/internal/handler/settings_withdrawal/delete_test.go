package settings_withdrawal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// deleteAccount はログイン中のユーザーとして、退会のフォームを送る。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
// GoはDELETEの本文をフォームとして読まないため、本文はPOSTのうちに MethodOverride が読んだものをハンドラーが使う。
func deleteAccount(t *testing.T, user *model.User, password string, confirmed bool, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{"_method": {http.MethodDelete}, "current_password": {password}}
	if confirmed {
		form.Set("confirmed", "1")
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/withdrawal", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(newHandler(t).Delete)).ServeHTTP(rec, req.WithContext(userContext(req, user)))

	return rec
}

// withdrawn はユーザーが退会したかを返す。
func withdrawn(t *testing.T, userID model.UserID) bool {
	t.Helper()

	user, err := repository.NewUserRepository(testutil.GetTestDB()).FindByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("ユーザーの取得のエラー = %v", err)
	}

	return user == nil
}

// cookieStates は応答がセッションのCookieを消したか・フラッシュメッセージを設定したかを返す。
func cookieStates(rec *httptest.ResponseRecorder) (sessionCleared, flashSet bool) {
	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case session.CookieName:
			sessionCleared = c.MaxAge < 0
		case session.FlashCookieName:
			flashSet = c.Value != ""
		}
	}

	return sessionCleared, flashSet
}

// TestDelete は、今のパスワードとチェックがそろえば退会させ、セッションの行とCookieを消して、
// 完了を伝えて利用者の言語のトップページへ送ることを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		locale       model.Locale
		wantLocation string
	}{
		{locale: model.LocaleJa, wantLocation: "/"},
		{locale: model.LocaleEn, wantLocation: "/en"},
	} {
		user := signedInUser(t, tt.locale)
		userSession := testutil.NewUserSessionBuilder(t, testutil.GetTestDB()).WithUserID(user.ID)
		userSession.Build()

		rec := deleteAccount(t, user, testutil.DefaultBuilderPassword, true, &http.Cookie{Name: session.CookieName, Value: userSession.Token()})

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Errorf("%s: 応答 = %d %q、期待値 = 303 %s", tt.locale, rec.Code, rec.Header().Get("Location"), tt.wantLocation)
		}
		sessionCleared, flashSet := cookieStates(rec)
		if !sessionCleared || !flashSet {
			t.Errorf("%s: セッションのCookieを消したか = %t・フラッシュメッセージ = %t、どちらもtrueを期待", tt.locale, sessionCleared, flashSet)
		}
		if !withdrawn(t, user.ID) {
			t.Errorf("%s: 退会していない", tt.locale)
		}
		var sessions int
		if err := testutil.GetTestDB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM user_sessions WHERE user_id = $1", uuid.UUID(user.ID)).Scan(&sessions); err != nil || sessions != 0 {
			t.Errorf("%s: セッションの行の数 = (%d, %v)、(0, nil) を期待", tt.locale, sessions, err)
		}
	}
}

// TestDelete_Invalid は、パスワードの誤りやチェックの無いフォームでは退会させず、画面を422で描き直し、
// 欄にエラーを結び付け、パスワードを戻さずチェックの状態だけを戻すことを検証する。
func TestDelete_Invalid(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)

	rec := deleteAccount(t, user, "wrong-password-value", true)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("パスワードの誤り: ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	assertContains(t, body, "パスワードが正しくありません", `aria-invalid="true"`, `value="1" required checked`)
	if strings.Contains(body, "wrong-password-value") {
		t.Error("入力したパスワードが描き直した画面に含まれている")
	}

	rec = deleteAccount(t, user, testutil.DefaultBuilderPassword, false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("チェックが無い: ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "退会を元に戻せないことを確認して、チェックを入れてください")

	if withdrawn(t, user.ID) {
		t.Error("退会した、退会しないことを期待")
	}
}

// TestDelete_RateLimited は、同じユーザーの試行を5回まで受け付け、6回目は合っていても照合せずに
// 429と Retry-After を返し、解除までの時間を示すことを検証する。
func TestDelete_RateLimited(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	for i := range 5 {
		if rec := deleteAccount(t, user, "wrong-password-value", true); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusUnprocessableEntity)
		}
	}

	rec := deleteAccount(t, user, testutil.DefaultBuilderPassword, true)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が設定されていない")
	}
	assertContains(t, rec.Body.String(), "試行の回数が上限に達しました。あと", "分で試せます")
	if withdrawn(t, user.ID) {
		t.Error("退会した、退会しないことを期待")
	}
}

// TestDelete_AlreadyWithdrawn は、二重送信で先の送信がパスワードとセッションを消した後も、
// 後の送信が完了を伝えずにCookieを消してトップページへ送ることを検証する。
func TestDelete_AlreadyWithdrawn(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	userSession := testutil.NewUserSessionBuilder(t, testutil.GetTestDB()).WithUserID(user.ID)
	userSession.Build()
	if first := deleteAccount(t, user, testutil.DefaultBuilderPassword, true, &http.Cookie{Name: session.CookieName, Value: userSession.Token()}); first.Code != http.StatusSeeOther {
		t.Fatalf("先の送信のステータスコード = %d、期待値 = %d", first.Code, http.StatusSeeOther)
	}
	var sessions int
	if err := testutil.GetTestDB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM user_sessions WHERE user_id = $1", uuid.UUID(user.ID)).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("退会後のセッションの行の数 = (%d, %v)、(0, nil) を期待", sessions, err)
	}

	rec := deleteAccount(t, user, testutil.DefaultBuilderPassword, true, &http.Cookie{Name: session.CookieName, Value: userSession.Token()})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("応答 = %d %q、期待値 = 303 /", rec.Code, rec.Header().Get("Location"))
	}
	sessionCleared, flashSet := cookieStates(rec)
	if !sessionCleared || flashSet {
		t.Errorf("セッションのCookieを消したか = %t・フラッシュメッセージ = %t、(true, false) を期待", sessionCleared, flashSet)
	}
}

// TestDelete_WithoutUser は、RequireAuth を通さずに届いたリクエストで誰も退会させないことを検証する。
func TestDelete_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodDelete, "/settings/withdrawal", strings.NewReader("current_password=x&confirmed=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler(t).Delete(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
