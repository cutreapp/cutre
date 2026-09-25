package account_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/account"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

const continuationKey = "test-continuation-token-key-0123456789"

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のデータベースに直接書き込む Handler を組み立てる。
// アカウントの作成のUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler() *account.Handler {
	db := testutil.GetTestDB()
	userRepo := repository.NewUserRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	invitationRedemptionRepo := repository.NewInvitationRedemptionRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	return account.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(continuationKey),
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		usecase.NewGetInvitationByIDUsecase(invitationRepo, invitationRedemptionRepo),
		usecase.NewGetConfirmedEmailConfirmationUsecase(emailConfirmationRepo),
		usecase.NewCreateAccountUsecase(
			db,
			invitationRepo,
			invitationRedemptionRepo,
			emailConfirmationRepo,
			validator.NewAccountCreateValidator(userRepo),
			userRepo,
			repository.NewUserPasswordRepository(db),
		),
		usecase.NewCreateSessionUsecase(userSessionRepo),
	)
}

// continuationCookies は招待と確認済みの確認のIDを運ぶCookieを返す。ゼロ値のIDのCookieは作らない。
func continuationCookies(invitationID model.InvitationID, confirmationID model.EmailConfirmationID) []*http.Cookie {
	rec := httptest.NewRecorder()
	mgr := session.NewContinuationManager(continuationKey)
	if invitationID != (model.InvitationID{}) {
		mgr.SetInvitationID(rec, invitationID)
	}
	if confirmationID != (model.EmailConfirmationID{}) {
		mgr.SetConfirmedEmailConfirmationID(rec, confirmationID)
	}

	return rec.Result().Cookies()
}

// newRequest はロケールとCookieを載せたリクエストを返す。
func newRequest(method, target string, form url.Values, locale string, cookies []*http.Cookie) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// responseCookie は応答が書き込んだ指定の名前のCookieを返す。無ければnilを返す。
func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	return nil
}

// confirmedEmail は確認を済ませた確認を作り、そのIDとメールアドレスを返す。
func confirmedEmail(t *testing.T) (model.EmailConfirmationID, string) {
	t.Helper()

	email := testutil.UniqueEmail("account-handler")
	id := testutil.NewEmailConfirmationBuilder(t, testutil.GetTestDB()).WithEmail(email).WithConfirmedAt(time.Now()).Build()

	return id, email
}

// TestNew は、両言語の入力画面に、確認したメールアドレスをログインの識別子として示し、
// 入力補助の属性とnoindexを付けることを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		path       string
		locale     string
		wantAction string
		wantLabel  string
	}{
		{path: "/account", locale: i18n.LangJa, wantAction: `action="/account"`, wantLabel: "アットネーム"},
		{path: "/en/account", locale: i18n.LangEn, wantAction: `action="/en/account"`, wantLabel: "Atname"},
	}
	for _, tt := range tests {
		confirmationID, email := confirmedEmail(t)

		rec := httptest.NewRecorder()
		newHandler().New(rec, newRequest(http.MethodGet, tt.path, nil, tt.locale, continuationCookies(testutil.NewInvitationBuilder(t, db).Build(), confirmationID)))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータス = %d、期待値 = 200", tt.path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{
			tt.wantAction, tt.wantLabel, `name="robots"`, `content="noindex`,
			`value="` + email + `" autocomplete="username" readonly`,
			`autocomplete="nickname"`, `maxlength="20"`, `pattern="[A-Za-z0-9_]+"`,
			`autocomplete="new-password"`, `minlength="8"`, `aria-describedby="password-hint"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: 応答に %q が無い", tt.path, want)
			}
		}
	}
}

// TestNew_RedirectsWithoutUsableContinuation は、招待か確認済みの確認を持たないとき、登録の画面へ戻すことを検証する。
func TestNew_RedirectsWithoutUsableContinuation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	tests := []struct {
		name           string
		invitationID   model.InvitationID
		confirmationID model.EmailConfirmationID
	}{
		{name: "確認無し", invitationID: testutil.NewInvitationBuilder(t, db).Build()},
		{name: "招待無し", confirmationID: testutil.NewEmailConfirmationBuilder(t, db).WithConfirmedAt(time.Now()).Build()},
		{
			name:           "未確認の確認",
			invitationID:   testutil.NewInvitationBuilder(t, db).Build(),
			confirmationID: testutil.NewEmailConfirmationBuilder(t, db).Build(),
		},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		newHandler().New(rec, newRequest(http.MethodGet, "/en/account", nil, i18n.LangEn, continuationCookies(tt.invitationID, tt.confirmationID)))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/en/sign_up" {
			t.Errorf("%s: 応答 = %d %q、303 /en/sign_up を期待", tt.name, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// TestNew_InvitationUnusable は、招待が使えなくなっているとき、その理由を403で示し、登録の途中のCookieを消すことを検証する。
func TestNew_InvitationUnusable(t *testing.T) {
	t.Parallel()

	confirmationID, _ := confirmedEmail(t)
	invitationID := testutil.NewInvitationBuilder(t, testutil.GetTestDB()).WithRevokedAt(time.Now()).Build()

	rec := httptest.NewRecorder()
	newHandler().New(rec, newRequest(http.MethodGet, "/account", nil, i18n.LangJa, continuationCookies(invitationID, confirmationID)))

	assertInvitationUnusable(t, rec)
}

// assertInvitationUnusable は、応答が使えない招待の案内を403で返し、招待と確認済みのCookieを消したことを確かめる。
func assertInvitationUnusable(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusForbidden {
		t.Errorf("ステータス = %d、期待値 = 403", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "この招待は使えなくなりました") || strings.Contains(body, `name="atname"`) {
		t.Error("フォームの代わりに、招待が使えなくなった案内を出していない")
	}
	for _, name := range []string{session.InvitationCookieName, session.ConfirmedEmailCookieName} {
		if cookie := responseCookie(rec, name); cookie == nil || cookie.MaxAge >= 0 {
			t.Errorf("%s = %+v、削除を期待", name, cookie)
		}
	}
}
