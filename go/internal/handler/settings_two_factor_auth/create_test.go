package settings_two_factor_auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// create はログイン中のユーザーとしてコードを送り、二要素認証を有効にする。
func create(t *testing.T, user *model.User, code string) *httptest.ResponseRecorder {
	t.Helper()

	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler(t).Create(rec, req.WithContext(userContext(req, user)))

	return rec
}

// preparedSecret は登録の画面を開いて登録の途中の設定を作り、その秘密鍵を返す。
func preparedSecret(t *testing.T, user *model.User) string {
	t.Helper()

	if rec := newPage(t, user); rec.Code != http.StatusOK {
		t.Fatalf("登録の画面のステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}

	return pendingSecret(t, user)
}

// recoveryCodePattern は画面に描いたリカバリーコード。
var recoveryCodePattern = regexp.MustCompile(`<li class="px-2 py-1.5">([a-z0-9]{4}-[a-z0-9]{4})</li>`)

// TestCreate は、正しいコードで二要素認証を有効にし、保存したリカバリーコードを一度だけリダイレクトせずに描画することを検証する。
// 戻るリンクは置かず、「保存しました」のチェックを求めるGETのフォームで二要素認証の画面へ進ませる。
func TestCreate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	code, err := totp.GenerateCode(preparedSecret(t, user), time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}

	rec := create(t, user, code)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>リカバリーコード | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		"二要素認証を有効にしました。",
		"このコードは、いまだけ表示されます。",
		`data-download-filename="cutre-recovery-codes.txt"`,
		`<form action="/settings/two_factor_auth" method="get"`,
		`<input type="checkbox" class="input" required>`,
	)
	assertNotContains(t, body, `aria-label="プロフィールにもどる"`, `aria-label="二要素認証にもどる"`)

	matches := recoveryCodePattern.FindAllStringSubmatch(body, -1)
	if len(matches) != auth.RecoveryCodeCount {
		t.Fatalf("描画したリカバリーコードの数 = %d、期待値 = %d", len(matches), auth.RecoveryCodeCount)
	}
	codes := make([]string, len(matches))
	for i, match := range matches {
		codes[i] = match[1]
	}
	text := strings.Join(codes, "\n") + "\n"
	assertContains(t, body, `data-copy-text="`+text+`"`, `data-download-text="`+text+`"`)

	twoFactorAuth, err := repository.NewUserTwoFactorAuthRepository(db).FindByUserID(ctx, user.ID)
	if err != nil || twoFactorAuth == nil || !twoFactorAuth.IsEnabled() {
		t.Errorf("二要素認証の設定 = (%+v, %v)、有効な設定を期待", twoFactorAuth, err)
	}
	digest := newTwoFactorKey(t).RecoveryCodeDigest(auth.NormalizeRecoveryCode(codes[0]))
	if used, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).Use(ctx, user.ID, digest); err != nil || !used {
		t.Errorf("描画したリカバリーコードの使用 = (%v, %v)、使えることを期待", used, err)
	}
}

// TestCreate_IncorrectCode は、コードが一致しなければ有効にせず、同じ秘密鍵のまま登録の画面を422で描き直し、
// 入力したコードを戻して入力欄のエラーを結び付けることを検証する。
func TestCreate_IncorrectCode(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	secret := preparedSecret(t, user)
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}
	incorrect := string('0'+(code[0]-'0'+1)%10) + code[1:]

	rec := create(t, user, incorrect)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(),
		`data-copy-text="`+secret+`"`,
		`value="`+incorrect+`"`,
		`aria-invalid="true"`,
		`aria-describedby="code-help code-`,
		"コードが正しくありません",
	)
	if got := pendingSecret(t, user); got != secret {
		t.Errorf("登録の途中の秘密鍵 = %q、描き直す前と同じ %q を期待", got, secret)
	}
}

// TestCreate_NotPending は、登録の途中の設定が無ければ (二重送信で既に有効にした場合を含む) 、
// 二要素認証の画面へ送ることを検証する。コードの形式の誤りでも、描き直す秘密鍵が無ければ同じく送る。
func TestCreate_NotPending(t *testing.T) {
	t.Parallel()

	for name, code := range map[string]string{"形式が正しい": "123456", "形式の誤り": "12345"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			user := signedInUser(t, model.LocaleJa)
			testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), user.ID).WithEnabledAt(time.Now()).Build()

			rec := create(t, user, code)

			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/two_factor_auth" {
				t.Errorf("応答 = %d %q、期待値 = 303 /settings/two_factor_auth", rec.Code, rec.Header().Get("Location"))
			}
		})
	}
}

// TestCreate_WithoutUser は、RequireAuth を通さずに届いたリクエストで二要素認証を有効にしないことを検証する。
func TestCreate_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/settings/two_factor_auth", nil)
	rec := httptest.NewRecorder()
	newHandler(t).Create(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
