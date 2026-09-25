package sign_in_two_factor_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

const (
	testContinuationKey = "test-continuation-token-key-0123456789"
	testTOTPKey         = "test-totp-encryption-key-0123456789"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.SetupTestMain(m))
}

// newHandler はテスト用のトランザクションの中で動く Handler を組み立てる。
// レート制限のカウンターも同じトランザクションで数え、テストの後に残さない。
func newHandler(t *testing.T) (*sign_in_two_factor.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	userSessionRepo := repository.NewUserSessionRepository(db).WithTx(tx)

	return sign_in_two_factor.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(testContinuationKey),
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx)),
		usecase.NewCreateSignInTwoFactorUsecase(
			newTwoFactorKey(t),
			validator.NewSignInTwoFactorCreateValidator(),
			repository.NewUserRepository(db).WithTx(tx),
			repository.NewUserTwoFactorAuthRepository(db).WithTx(tx),
		),
		usecase.NewCreateSessionUsecase(userSessionRepo),
	), tx
}

func newTwoFactorKey(t *testing.T) *auth.TwoFactorKey {
	t.Helper()

	key, err := auth.NewTwoFactorKey(testTOTPKey)
	if err != nil {
		t.Fatalf("鍵の作成のエラー = %v", err)
	}
	return key
}

// createTwoFactorUser は二要素認証を有効にしたユーザーを作り、そのIDと秘密鍵を返す。
func createTwoFactorUser(t *testing.T, tx *sql.Tx, locale model.Locale) (model.UserID, string) {
	t.Helper()

	userID := testutil.NewUserBuilder(t, tx).WithLocale(locale).Build()
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("秘密鍵の生成のエラー = %v", err)
	}
	id := uuid.UUID(userID)
	ciphertext, err := newTwoFactorKey(t).EncryptTOTPSecret(secret, id[:])
	if err != nil {
		t.Fatalf("秘密鍵の暗号化のエラー = %v", err)
	}
	testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).WithSecretCiphertext(ciphertext).WithEnabledAt(time.Now()).Build()

	return userID, secret
}

// currentCode は秘密鍵の今のコードを返す。
func currentCode(t *testing.T, secret string) string {
	t.Helper()

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}
	return code
}

// pendingCookie は、パスワードを確かめたユーザーとして userID を運ぶCookieを返す。
func pendingCookie(userID model.UserID) *http.Cookie {
	rec := httptest.NewRecorder()
	session.NewContinuationManager(testContinuationKey).SetTwoFactorPendingUserID(rec, userID)
	return rec.Result().Cookies()[0]
}

// TestNew は、コードの入力画面を表示中の言語版で描画し、戻り先をフォームとリンクへ引き継ぐことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		locale string
		target string
		want   []string
	}{
		{
			name:   "日本語",
			locale: i18n.LangJa,
			target: "/sign_in/two_factor",
			want: []string{
				`action="/sign_in/two_factor"`,
				"確認コードを入力",
				`href="/sign_in/two_factor/recovery"`,
				`href="/sign_in"`,
				"ヘルプからお問い合わせ",
			},
		},
		{
			name:   "英語で戻り先あり",
			locale: i18n.LangEn,
			target: "/en/sign_in/two_factor?return_to=%2Fsettings%2Finvitation",
			want: []string{
				`action="/en/sign_in/two_factor"`,
				"Enter your verification code",
				`name="return_to" value="/settings/invitation"`,
				`href="/en/sign_in/two_factor/recovery?return_to=%2Fsettings%2Finvitation"`,
				`href="/en/sign_in?return_to=%2Fsettings%2Finvitation"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, _ := newHandler(t)
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req = req.WithContext(i18n.SetLocale(req.Context(), tt.locale))
			req.AddCookie(pendingCookie(model.UserID(uuid.New())))
			rec := httptest.NewRecorder()
			handler.New(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			for _, want := range append(tt.want, `content="noindex`, `autocomplete="one-time-code"`, `inputmode="numeric"`, "data-clear-on-history-restore") {
				if !strings.Contains(body, want) {
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}

// TestNew_WithoutPendingCookie は、パスワードを確かめたCookieが無い・偽造されているときに、
// 画面を描画せずログイン画面へ戻り先を引き継いで送ることを検証する。
func TestNew_WithoutPendingCookie(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{
		"Cookie無し": "",
		"署名を偽った値":  "v1.two_factor_pending." + uuid.NewString() + ".9999999999.forged",
	} {
		handler, _ := newHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/en/sign_in/two_factor?return_to=%2Fhome", nil)
		req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangEn))
		if value != "" {
			req.AddCookie(&http.Cookie{Name: session.TwoFactorPendingCookieName, Value: value})
		}
		rec := httptest.NewRecorder()
		handler.New(rec, req)

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/en/sign_in?return_to=%2Fhome" {
			t.Errorf("%s: 応答 = %d %q、期待値 = 303 %q", name, rec.Code, rec.Header().Get("Location"), "/en/sign_in?return_to=%2Fhome")
		}
	}
}
