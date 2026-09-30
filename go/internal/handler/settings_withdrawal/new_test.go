package settings_withdrawal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
	tradeRepo := repository.NewTradeRepository(db)

	return settings_withdrawal.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewGetWithdrawalUsecase(tradeRepo),
		usecase.NewDeleteAccountUsecase(
			db,
			validator.NewWithdrawalDeleteValidator(userPasswordRepo, tradeRepo),
			repository.NewUserRepository(db),
			userPasswordRepo,
			userSessionRepo,
			repository.NewUserTwoFactorAuthRepository(db),
			repository.NewUserTwoFactorRecoveryCodeRepository(db),
			repository.NewPasswordResetTokenRepository(db),
			repository.NewEmailConfirmationRepository(db),
			repository.NewInvitationRepository(db),
			repository.NewItemRepository(db),
			repository.NewUserStationRepository(db),
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
	// マイページから辿った画面のため、メインメニューのマイページの項目を選択中 (aria-current="true") にする。
	// ホームの項目を選択中にしても aria-current="true" は出るため、マイページのリンクに付いていることまで確かめる。
	myPageLink := regexp.MustCompile(`href="/@` + regexp.QuoteMeta(user.Atname) + `" class="[^"]*" aria-current="true"`)
	if !myPageLink.MatchString(rec.Body.String()) {
		t.Error("メインメニューのマイページの項目に aria-current=\"true\" が付いていない")
	}
	assertContains(t, rec.Body.String(),
		"<title>退会 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`<html lang="ja" data-main-nav>`,
		`<nav aria-label="パンくずリスト">`,
		`href="/@`+user.Atname+`"`,
		`<h1 class="text-xl font-semibold">退会</h1>`,
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

// TestNew_TradeInProgress は、進行中の交換があるとき、その数と交換の画面への案内を出し、
// 退会するボタンを押せなくして、押せない理由をボタンの説明として結び付けることを検証する。
func TestNew_TradeInProgress(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	partnerID := testutil.NewUserBuilder(t, db).Build()
	testutil.NewTradeBuilder(t, db, user.ID, partnerID).Build()
	testutil.NewTradeBuilder(t, db, partnerID, user.ID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, db, user.ID, partnerID).WithStatus(model.TradeStatusCompleted).Build()

	req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal", nil)
	rec := httptest.NewRecorder()
	newHandler(t).New(rec, req.WithContext(userContext(req, user)))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		`<div class="alert" data-variant="warning">`,
		"進行中の交換が2件あります。すべて終わるまで退会できません",
		`href="/trades"`,
		"進行中の交換を見る",
		`disabled aria-describedby="withdrawal-submit-hint"`,
		`<p id="withdrawal-submit-hint"`,
		"進行中の交換がすべて終わると押せます",
	)
}

// TestNew_WithoutTradeInProgress は、終わった交換しか無いときは、退会できない案内を出さず、退会するボタンを押せることを検証する。
func TestNew_WithoutTradeInProgress(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	user := signedInUser(t, model.LocaleJa)
	testutil.NewTradeBuilder(t, db, user.ID, testutil.NewUserBuilder(t, db).Build()).WithStatus(model.TradeStatusCompleted).Build()

	req := httptest.NewRequest(http.MethodGet, "/settings/withdrawal", nil)
	rec := httptest.NewRecorder()
	newHandler(t).New(rec, req.WithContext(userContext(req, user)))

	body := rec.Body.String()
	assertContains(t, body, "リストと交換場所は消えます", "あなたの名前は「退会したユーザー」になり")
	for _, unwanted := range []string{"進行中の交換が", "disabled", "withdrawal-submit-hint"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("レスポンスボディに %q が含まれている", unwanted)
		}
	}
}
