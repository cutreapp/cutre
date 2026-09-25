package sign_up_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/handler/sign_up"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

const testContinuationKey = "test-continuation-token-key-0123456789"

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のデータベースに直接書き込む Handler を組み立てる。
// 登録のUseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newHandler(t *testing.T, turnstileVerifier *testutil.FakeTurnstileVerifier) *sign_up.Handler {
	t.Helper()

	db := testutil.GetTestDB()
	jobs, err := dispatcher.NewDispatcher(db)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}

	return sign_up.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(testContinuationKey),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		turnstileVerifier,
		usecase.NewGetInvitationByIDUsecase(repository.NewInvitationRepository(db), repository.NewInvitationRedemptionRepository(db)),
		usecase.NewCreateSignUpUsecase(
			db,
			repository.NewInvitationRepository(db),
			repository.NewInvitationRedemptionRepository(db),
			validator.NewSignUpCreateValidator(repository.NewUserRepository(db)),
			repository.NewEmailConfirmationRepository(db),
			jobs,
		),
	)
}

// invitationCookie は、使える招待を作ってそのIDを運ぶCookieを返す。
func invitationCookie(t *testing.T, builder *testutil.InvitationBuilder) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	session.NewContinuationManager(testContinuationKey).SetInvitationID(rec, builder.Build())

	return rec.Result().Cookies()[0]
}

// newRequest はロケールを載せたリクエストを返す。remoteAddrはレート制限を数える単位を他のテストと分けるために渡す。
func newRequest(method, target string, form url.Values, locale string, remoteAddr string, cookies ...*http.Cookie) *http.Request {
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

// TestNew は、使える招待を持つ人にはメールアドレスの入力フォームを、持たない人には招待制の案内を描画することを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	db := testutil.GetTestDB()

	tests := []struct {
		name         string
		target       string
		locale       string
		cookie       *http.Cookie
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:   "使える招待を持つ",
			target: "/sign_up",
			locale: i18n.LangJa,
			cookie: invitationCookie(t, testutil.NewInvitationBuilder(t, db)),
			wantContains: []string{
				`action="/sign_up"`,
				`autocomplete="email"`,
				`aria-describedby="email-hint"`,
				`data-disable-on-submit`,
				`<link rel="canonical"`,
			},
			wantAbsent: []string{"Cutreは招待制です", `name="robots"`},
		},
		{
			name:         "英語版",
			target:       "/en/sign_up",
			locale:       i18n.LangEn,
			cookie:       invitationCookie(t, testutil.NewInvitationBuilder(t, db)),
			wantContains: []string{`<html lang="en">`, `action="/en/sign_up"`, "Send confirmation code"},
		},
		{
			name:         "招待を持たない",
			target:       "/sign_up",
			locale:       i18n.LangJa,
			wantContains: []string{"Cutreは招待制です"},
			wantAbsent:   []string{`method="post"`},
		},
		{
			name:         "使えなくなった招待を持つ",
			target:       "/sign_up",
			locale:       i18n.LangJa,
			cookie:       invitationCookie(t, testutil.NewInvitationBuilder(t, db).WithRevokedAt(time.Now())),
			wantContains: []string{"Cutreは招待制です"},
			wantAbsent:   []string{`method="post"`},
		},
	}

	for _, tt := range tests {
		cookies := []*http.Cookie{}
		if tt.cookie != nil {
			cookies = append(cookies, tt.cookie)
		}
		rec := httptest.NewRecorder()
		handler.New(rec, newRequest(http.MethodGet, tt.target, nil, tt.locale, "192.0.2.30:1234", cookies...))

		if rec.Code != http.StatusOK {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		for _, want := range tt.wantContains {
			if !strings.Contains(body, want) {
				t.Errorf("%s: レスポンスボディに %q が含まれていない", tt.name, want)
			}
		}
		for _, absent := range tt.wantAbsent {
			if strings.Contains(body, absent) {
				t.Errorf("%s: レスポンスボディに %q が含まれている", tt.name, absent)
			}
		}
	}
}

// TestNew_DeletesUnusableInvitationCookie は、使えなくなった招待のCookieを削除することを検証する。
func TestNew_DeletesUnusableInvitationCookie(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	cookie := invitationCookie(t, testutil.NewInvitationBuilder(t, testutil.GetTestDB()).WithExpiresAt(time.Now().Add(-time.Minute)))

	rec := httptest.NewRecorder()
	handler.New(rec, newRequest(http.MethodGet, "/sign_up", nil, i18n.LangJa, "192.0.2.31:1234", cookie))

	deleted := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.InvitationCookieName && c.MaxAge < 0 {
			deleted = true
		}
	}
	if !deleted {
		t.Error("使えなくなった招待のCookieを削除していない")
	}
}

// assertEmailConfirmationCookie は、応答が確認のIDを運ぶCookieを書き込んだことを確かめる。
func assertEmailConfirmationCookie(t *testing.T, rec *httptest.ResponseRecorder) model.EmailConfirmationID {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/email_confirmation", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	id, ok := session.NewContinuationManager(testContinuationKey).EmailConfirmationID(req)
	if !ok {
		t.Fatal("確認のIDを運ぶCookieが書き込まれていない")
	}

	return id
}

// assertInvitationCookie は、応答が招待のCookieを同じ招待のIDで発行し直したことを確かめる。
func assertInvitationCookie(t *testing.T, rec *httptest.ResponseRecorder, want model.InvitationID) {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name != session.InvitationCookieName {
			continue
		}
		req := httptest.NewRequest(http.MethodGet, "/sign_up", nil)
		req.AddCookie(c)
		if got, ok := session.NewContinuationManager(testContinuationKey).InvitationID(req); !ok || got != want {
			t.Errorf("招待のCookieのID = (%v, %t)、期待値 = (%v, true)", got, ok, want)
		}
		return
	}
	t.Fatal("招待のCookieが発行し直されていない")
}
