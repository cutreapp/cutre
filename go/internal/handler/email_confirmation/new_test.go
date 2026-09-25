package email_confirmation_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/handler/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
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
// 再送が使う登録のUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler(t *testing.T) *email_confirmation.Handler {
	t.Helper()

	db := testutil.GetTestDB()
	jobs, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	return email_confirmation.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(continuationKey),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewGetInvitationByIDUsecase(repository.NewInvitationRepository(db), repository.NewInvitationRedemptionRepository(db)),
		usecase.NewGetEmailConfirmationUsecase(emailConfirmationRepo),
		usecase.NewVerifyEmailConfirmationUsecase(validator.NewEmailConfirmationCreateValidator(), emailConfirmationRepo),
		usecase.NewCreateSignUpUsecase(
			db,
			repository.NewInvitationRepository(db),
			repository.NewInvitationRedemptionRepository(db),
			validator.NewSignUpCreateValidator(repository.NewUserRepository(db)),
			emailConfirmationRepo,
			jobs,
		),
	)
}

// continuationCookies は招待と確認のIDを運ぶCookieを返す。ゼロ値のIDのCookieは作らない。
func continuationCookies(invitationID model.InvitationID, confirmationID model.EmailConfirmationID) []*http.Cookie {
	rec := httptest.NewRecorder()
	mgr := session.NewContinuationManager(continuationKey)
	if invitationID != (model.InvitationID{}) {
		mgr.SetInvitationID(rec, invitationID)
	}
	if confirmationID != (model.EmailConfirmationID{}) {
		mgr.SetEmailConfirmationID(rec, confirmationID)
	}

	return rec.Result().Cookies()
}

// newRequest はロケールを載せたリクエストを返す。remoteAddrはレート制限を数える単位を他のテストと分けるために渡す。
func newRequest(method, target string, form url.Values, locale, remoteAddr string, cookies []*http.Cookie) *http.Request {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.RemoteAddr = remoteAddr
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

// TestNew はコード入力画面の日本語・英語、入力補助属性とnoindexを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	id := testutil.NewInvitationBuilder(t, testutil.GetTestDB()).Build()
	tests := []struct {
		path       string
		locale     string
		wantAction string
		wantLabel  string
	}{
		{path: "/email_confirmation", locale: i18n.LangJa, wantAction: `action="/email_confirmation"`, wantLabel: "確認コード"},
		{path: "/en/email_confirmation", locale: i18n.LangEn, wantAction: `action="/en/email_confirmation"`, wantLabel: "Confirmation code"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		newHandler(t).New(rec, newRequest(http.MethodGet, tt.path, nil, tt.locale, "192.0.2.60:1234", continuationCookies(id, model.EmailConfirmationID(uuid.New()))))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータス = %d、期待値 = 200", tt.path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{tt.wantAction, tt.wantLabel, `name="robots"`, `content="noindex`, `autocomplete="one-time-code"`, `inputmode="numeric"`, `pattern="[0-9]{6}"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: 応答に %q が無い", tt.path, want)
			}
		}
	}
}

// TestNew_RedirectsWithoutUsableContinuation は確認または招待の継続情報が無いとき登録画面へ戻す。
func TestNew_RedirectsWithoutUsableContinuation(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	id := testutil.NewInvitationBuilder(t, db).Build()
	revokedID := testutil.NewInvitationBuilder(t, db).WithRevokedAt(time.Now()).Build()
	tests := []struct {
		name             string
		invitationID     model.InvitationID
		withConfirmation bool
	}{
		{name: "確認無し", invitationID: id},
		{name: "招待無し", withConfirmation: true},
		{name: "取り消し済みの招待", invitationID: revokedID, withConfirmation: true},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		var confirmationID model.EmailConfirmationID
		if tt.withConfirmation {
			confirmationID = model.EmailConfirmationID(uuid.New())
		}
		newHandler(t).New(rec, newRequest(http.MethodGet, "/email_confirmation", nil, i18n.LangJa, "192.0.2.60:1234", continuationCookies(tt.invitationID, confirmationID)))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/sign_up" {
			t.Errorf("%s: 応答 = %d %q、303 /sign_up を期待", tt.name, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// assertInvitationCookie は、応答が招待のCookieを同じ招待のIDで発行し直したことを確かめる。
func assertInvitationCookie(t *testing.T, rec *httptest.ResponseRecorder, want model.InvitationID) {
	t.Helper()

	cookie := responseCookie(rec, session.InvitationCookieName)
	if cookie == nil {
		t.Fatal("招待のCookieが発行し直されていない")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	if got, ok := session.NewContinuationManager(continuationKey).InvitationID(req); !ok || got != want {
		t.Errorf("招待のCookieのID = (%v, %t)、期待値 = (%v, true)", got, ok, want)
	}
}
