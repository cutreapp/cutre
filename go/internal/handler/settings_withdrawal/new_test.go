package settings_withdrawal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/settings_withdrawal"
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

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のデータベースに直接書き込む Handler を返す。
// 退会のUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler(t *testing.T) *settings_withdrawal.Handler {
	t.Helper()

	db := testutil.GetTestDB()
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	return settings_withdrawal.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewDeleteAccountUsecase(
			db,
			validator.NewWithdrawalDeleteValidator(userPasswordRepo),
			repository.NewUserRepository(db),
			userPasswordRepo,
			userSessionRepo,
			repository.NewUserTwoFactorAuthRepository(db),
			repository.NewUserTwoFactorRecoveryCodeRepository(db),
			repository.NewPasswordResetTokenRepository(db),
			repository.NewEmailConfirmationRepository(db),
			repository.NewInvitationRepository(db),
		),
	)
}

// signedInUser は、既定のパスワードを持つテスト用のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func signedInUser(t *testing.T, locale model.Locale) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).WithLocale(locale).Build()
	testutil.NewUserPasswordBuilder(t, db).WithUserID(id).Build()
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

// assertContains はbodyがすべての文字列を含むことを確かめる。
func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("レスポンスボディに %q が含まれていない", want)
		}
	}
}

// TestNew は、退会の画面に、退会すると何が起きるかの説明と、今のパスワード・理解したことのチェックを求めるフォームを描くことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	const csrfToken = "withdrawal-csrf-token"
	user := signedInUser(t, model.LocaleJa)
	req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
	rec := httptest.NewRecorder()
	middleware.NewCSRF().Middleware(http.HandlerFunc(newHandler(t).New)).ServeHTTP(rec, req.WithContext(userContext(req, user)))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>退会 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`href="/@`+user.Atname+`"`,
		"退会すると",
		"あなたの招待リンクは使えなくなります",
		`action="/settings/withdrawal" method="post"`,
		`name="_method" value="DELETE"`,
		`name="csrf_token" value="`+csrfToken+`"`,
		`name="current_password" type="password" autocomplete="current-password" required`,
		`name="confirmed" type="checkbox" class="input" value="1" required`,
		"退会を元に戻せないことを理解しました",
		"退会する",
	)
}

// TestNew_WithoutUser は、RequireAuth を通さずに届いたリクエストで画面を描かないことを検証する。
func TestNew_WithoutUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal", nil)
	rec := httptest.NewRecorder()
	newHandler(t).New(rec, req.WithContext(i18n.SetLocale(req.Context(), i18n.LangJa)))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
