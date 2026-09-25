package settings_two_factor_auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// deleteTwoFactorAuth はログイン中のユーザーとして、再認証の入力を送って二要素認証を無効にする。
// ブラウザのフォームと同じくPOSTに _method を載せて送り、MethodOverride を通す。
// GoはDELETEの本文をフォームとして読まないため、本文はPOSTのうちに MethodOverride が読んだものをハンドラーが使う。
func deleteTwoFactorAuth(t *testing.T, user *model.User, credential string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{"_method": {http.MethodDelete}, "credential": {credential}}
	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	middleware.MethodOverride(http.HandlerFunc(newHandler(t).Delete)).ServeHTTP(rec, req.WithContext(userContext(req, user)))

	return rec
}

// enabledUser は、既定のパスワードと有効な二要素認証の設定を持つログイン中のユーザーを作り、ユーザーと秘密鍵を返す。
func enabledUser(t *testing.T) (*model.User, string) {
	t.Helper()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	testutil.NewUserPasswordBuilder(t, db).WithUserID(user.ID).Build()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("秘密鍵の生成のエラー = %v", err)
	}
	id := uuid.UUID(user.ID)
	ciphertext, err := newTwoFactorKey(t).EncryptTOTPSecret(secret, id[:])
	if err != nil {
		t.Fatalf("秘密鍵の暗号化のエラー = %v", err)
	}
	testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).WithSecretCiphertext(ciphertext).WithEnabledAt(time.Now()).Build()

	return user, secret
}

// twoFactorAuthEnabled はユーザーの二要素認証が有効なままかを返す。
func twoFactorAuthEnabled(t *testing.T, userID model.UserID) bool {
	t.Helper()

	setting, err := repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()).FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("二要素認証の設定の取得のエラー = %v", err)
	}

	return setting != nil && setting.IsEnabled()
}

// flashSet は応答がフラッシュメッセージを設定したかを返す。
func flashSet(rec *httptest.ResponseRecorder) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return true
		}
	}

	return false
}

// TestDelete は、今のパスワードか認証アプリのコードで再認証できれば二要素認証を無効にし、
// 完了を伝えて二要素認証の画面へ戻すことを検証する。
func TestDelete(t *testing.T) {
	t.Parallel()

	byPassword, _ := enabledUser(t)
	byCode, secret := enabledUser(t)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}

	for name, tt := range map[string]struct {
		user       *model.User
		credential string
	}{
		"パスワード":     {user: byPassword, credential: testutil.DefaultBuilderPassword},
		"認証アプリのコード": {user: byCode, credential: code},
	} {
		rec := deleteTwoFactorAuth(t, tt.user, tt.credential)

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/two_factor_auth" {
			t.Errorf("%s: 応答 = %d %q、期待値 = 303 /settings/two_factor_auth", name, rec.Code, rec.Header().Get("Location"))
		}
		if !flashSet(rec) {
			t.Errorf("%s: フラッシュメッセージが設定されていない", name)
		}
		if twoFactorAuthEnabled(t, tt.user.ID) {
			t.Errorf("%s: 二要素認証が有効なまま", name)
		}
	}
}

// TestDelete_Incorrect は、再認証の入力が合わなければ、二要素認証を残したまま画面を422で描き直し、
// 入力の欄にエラーを結び付け、入力した値を戻さないことを検証する。
func TestDelete_Incorrect(t *testing.T) {
	t.Parallel()

	user, _ := enabledUser(t)

	rec := deleteTwoFactorAuth(t, user, "wrong-password-value")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	assertContains(t, body, "二要素認証はオンです", "パスワードまたはコードが正しくありません", `aria-invalid="true"`)
	assertNotContains(t, body, "wrong-password-value")
	if !twoFactorAuthEnabled(t, user.ID) {
		t.Error("二要素認証が無効になった、有効なままを期待")
	}
}

// TestDelete_RateLimited は、同じユーザーの再認証を5回まで受け付け、6回目は合っていても照合せずに
// 429と Retry-After を返し、解除までの時間を示すことを検証する。
func TestDelete_RateLimited(t *testing.T) {
	t.Parallel()

	user, _ := enabledUser(t)
	for i := range 5 {
		if rec := deleteTwoFactorAuth(t, user, "wrong-password-value"); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusUnprocessableEntity)
		}
	}

	rec := deleteTwoFactorAuth(t, user, testutil.DefaultBuilderPassword)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が設定されていない")
	}
	assertContains(t, rec.Body.String(), "試行の回数が上限に達しました。あと", "分で試せます")
	if !twoFactorAuthEnabled(t, user.ID) {
		t.Error("二要素認証が無効になった、有効なままを期待")
	}
}

// TestDelete_NotEnabled は、二要素認証を有効にしていなければ (別の画面で既に無効にした)、
// 完了を伝えずに二要素認証の画面へ戻すことを検証する。
func TestDelete_NotEnabled(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	testutil.NewUserPasswordBuilder(t, testutil.GetTestDB()).WithUserID(user.ID).Build()

	rec := deleteTwoFactorAuth(t, user, testutil.DefaultBuilderPassword)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/two_factor_auth" {
		t.Errorf("応答 = %d %q、期待値 = 303 /settings/two_factor_auth", rec.Code, rec.Header().Get("Location"))
	}
	if flashSet(rec) {
		t.Error("フラッシュメッセージが設定された、設定しないことを期待")
	}
}

// TestDelete_WithoutUser は、RequireAuth を通さずに届いたリクエストで誰の二要素認証も無効にしないことを検証する。
func TestDelete_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodDelete, "/settings/two_factor_auth", strings.NewReader("credential=123456"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler(t).Delete(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
