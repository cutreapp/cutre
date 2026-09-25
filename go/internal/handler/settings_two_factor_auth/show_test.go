package settings_two_factor_auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newTwoFactorKey はテスト用の鍵の TwoFactorKey を返す。
func newTwoFactorKey(t *testing.T) *auth.TwoFactorKey {
	t.Helper()

	key, err := auth.NewTwoFactorKey("test-totp-encryption-key-0123456789")
	if err != nil {
		t.Fatalf("鍵の作成のエラー = %v", err)
	}

	return key
}

// newHandler はテスト用のデータベースに直接書き込む Handler を返す。
// 二要素認証を有効・無効にするUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler(t *testing.T) *settings_two_factor_auth.Handler {
	t.Helper()

	db := testutil.GetTestDB()
	key := newTwoFactorKey(t)
	twoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	recoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)

	return settings_two_factor_auth.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewGetTwoFactorAuthStatusUsecase(twoFactorAuthRepo, recoveryCodeRepo),
		usecase.NewPrepareTwoFactorAuthUsecase(key, twoFactorAuthRepo),
		usecase.NewGetPendingTwoFactorAuthUsecase(key, twoFactorAuthRepo),
		usecase.NewEnableTwoFactorAuthUsecase(db, key, validator.NewTwoFactorAuthCreateValidator(), twoFactorAuthRepo, recoveryCodeRepo),
		usecase.NewDisableTwoFactorAuthUsecase(
			db,
			key,
			validator.NewTwoFactorAuthDeleteValidator(),
			repository.NewUserPasswordRepository(db),
			twoFactorAuthRepo,
			recoveryCodeRepo,
		),
	)
}

// signedInUser はテスト用のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func signedInUser(t *testing.T, locale model.Locale) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).WithLocale(locale).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// userContext はログイン中のユーザーとロケールを載せたcontextを返す。ロケールは本番では UserLocale が users.locale から決める。
func userContext(req *http.Request, user *model.User) context.Context {
	return middleware.SetUserToContext(i18n.SetLocale(req.Context(), string(user.Locale)), user)
}

// show はログイン中のユーザーとして二要素認証の画面を開く。
func show(t *testing.T, user *model.User) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth", nil)
	rec := httptest.NewRecorder()
	newHandler(t).Show(rec, req.WithContext(userContext(req, user)))

	return rec
}

// assertContains はボディに want のすべてが含まれることを検証する。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// assertNotContains はボディに absent のいずれも含まれないことを検証する。
func assertNotContains(t *testing.T, body string, absents ...string) {
	t.Helper()

	for _, absent := range absents {
		if strings.Contains(body, absent) {
			t.Errorf("レスポンスボディに %q が含まれている", absent)
		}
	}
}

// TestShow_Disabled は、二要素認証を有効にしていなければ、有効にしたときの変化と有効にする画面への入口を描画することを検証する。
// 認証アプリへの登録の途中の設定も、まだ有効にしていないものとして描く。
func TestShow_Disabled(t *testing.T) {
	t.Parallel()

	user := signedInUser(t, model.LocaleJa)
	testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), user.ID).Build()

	rec := show(t, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>二要素認証 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`href="/@`+user.Atname+`"`,
		"二要素認証はオフです",
		"有効にすると",
		`href="/settings/two_factor_auth/new"`,
	)
	assertNotContains(t, body, "二要素認証はオンです", "件 残っています", `name="credential"`)
}

// TestShow_Enabled は、二要素認証を有効にしていれば、有効にした日 (ユーザーのタイムゾーンの日付) と
// 未使用のリカバリーコードの数を描画し、有効にする画面への入口を出さないことを検証する。
func TestShow_Enabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	tokyo, _ := time.LoadLocation(model.DefaultTimeZone)
	// 東京では1月2日、UTCでは1月1日にあたる時刻。
	enabledAt := time.Date(time.Now().In(tokyo).Year(), 1, 2, 0, 30, 0, 0, tokyo)
	testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).WithEnabledAt(enabledAt).Build()
	recoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	if err := recoveryCodeRepo.CreateAll(ctx, user.ID, []string{"digest-1", "digest-2", "digest-3"}); err != nil {
		t.Fatalf("リカバリーコードの作成のエラー = %v", err)
	}
	if _, err := recoveryCodeRepo.Use(ctx, user.ID, "digest-1"); err != nil {
		t.Fatalf("リカバリーコードの使用のエラー = %v", err)
	}

	rec := show(t, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "二要素認証はオンです", "1月2日に有効にしました", "設定済み", "2件 残っています",
		"無効にする",
		`<input type="hidden" name="_method" value="DELETE">`,
		`name="credential" type="password" autocomplete="current-password" required`,
		"二要素認証を無効にする",
	)
	assertNotContains(t, body, "二要素認証はオフです", `href="/settings/two_factor_auth/new"`)
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かの設定として描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/settings/two_factor_auth", nil)
	rec := httptest.NewRecorder()
	newHandler(t).Show(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
