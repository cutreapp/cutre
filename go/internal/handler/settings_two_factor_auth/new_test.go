package settings_two_factor_auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newPage はログイン中のユーザーとして二要素認証を有効にする画面を開く。
func newPage(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil)
	rec := httptest.NewRecorder()
	newHandler(t).New(rec, req.WithContext(userContext(req, user)))

	return rec
}

// pendingSecret は登録の途中の秘密鍵を返す。
func pendingSecret(t *testing.T, user *model.User) string {
	t.Helper()

	uc := usecase.NewGetPendingTwoFactorAuthUsecase(newTwoFactorKey(t), repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()))
	output, err := uc.Execute(context.Background(), usecase.GetPendingTwoFactorAuthInput{User: user})
	if err != nil || output.Setup == nil {
		t.Fatalf("登録の途中の秘密鍵 = (%+v, %v)、秘密鍵を期待", output, err)
	}

	return output.Setup.Secret
}

// TestNew は、新しい秘密鍵を登録の途中の設定に保存し、otpauthのリンク・QRコード・手入力用のキーと、
// コードを入力するフォームを描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	rec := newPage(t, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	secret := pendingSecret(t, user)
	body := rec.Body.String()
	assertContains(t, body,
		"<title>二要素認証を有効にする | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`href="/settings/two_factor_auth"`,
		`href="otpauth://totp/Cutre:`+user.Atname+`?`,
		"secret="+secret,
		`role="img" aria-label="認証アプリに追加するQRコード"`,
		`value="`+secret[:4]+" "+secret[4:8]+" ",
		`data-copy-text="`+secret+`"`,
		`<form action="/settings/two_factor_auth" method="post"`,
		`name="csrf_token"`,
		`name="code"`,
		`inputmode="numeric"`,
		`autocomplete="one-time-code"`,
		`aria-describedby="code-help"`,
	)
	assertNotContains(t, body, `aria-invalid="true"`)
}

// TestNew_Enabled は、既に有効にしていれば秘密鍵を差し替えず、二要素認証の画面へ送ることを検証する。
func TestNew_Enabled(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), user.ID).WithEnabledAt(time.Now()).Build()

	rec := newPage(t, user)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/two_factor_auth" {
		t.Errorf("応答 = %d %q、期待値 = 303 /settings/two_factor_auth", rec.Code, rec.Header().Get("Location"))
	}
}

// TestNew_WithoutUser は、RequireAuth を通さずに届いたリクエストで秘密鍵を作らないことを検証する。
func TestNew_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth/new", nil)
	rec := httptest.NewRecorder()
	newHandler(t).New(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
