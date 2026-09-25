package sign_in_two_factor_recovery_test

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/sign_in_two_factor_recovery"
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

// newHandler は Handler を組み立てる。
// UseCaseがコードの消費とセッションの作成のトランザクションを自前で開くため、テスト用のトランザクションではなく
// GetTestDB の接続を使う (テストデータはコミットされる)。
// レート制限のカウンターも残るため、送信ごとに uniqueRemoteAddr で接続元を変え、ユーザーも毎回作る。
func newHandler(t *testing.T) (*sign_in_two_factor_recovery.Handler, *sql.DB) {
	t.Helper()

	db := testutil.GetTestDB()
	userSessionRepo := repository.NewUserSessionRepository(db)

	return sign_in_two_factor_recovery.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewContinuationManager(testContinuationKey),
		session.NewManager(userSessionRepo),
		session.NewFlashManager(),
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		usecase.NewCreateSignInTwoFactorRecoveryUsecase(
			db,
			newTwoFactorKey(t),
			validator.NewSignInTwoFactorRecoveryCreateValidator(),
			repository.NewUserRepository(db),
			repository.NewUserTwoFactorAuthRepository(db),
			repository.NewUserTwoFactorRecoveryCodeRepository(db),
			userSessionRepo,
		),
	), db
}

func newTwoFactorKey(t *testing.T) *auth.TwoFactorKey {
	t.Helper()

	key, err := auth.NewTwoFactorKey(testTOTPKey)
	if err != nil {
		t.Fatalf("鍵の作成のエラー = %v", err)
	}
	return key
}

// createTwoFactorUser は二要素認証を有効にしたユーザーを作り、リカバリーコード code を1つ持たせてIDを返す。
func createTwoFactorUser(t *testing.T, db *sql.DB, locale model.Locale, code string) model.UserID {
	t.Helper()

	userID := testutil.NewUserBuilder(t, db).WithLocale(locale).Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db, userID).WithEnabledAt(time.Now()).Build()
	digest := newTwoFactorKey(t).RecoveryCodeDigest(auth.NormalizeRecoveryCode(code))
	if err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CreateAll(t.Context(), userID, []string{digest}); err != nil {
		t.Fatalf("リカバリーコードの作成のエラー = %v", err)
	}

	return userID
}

// pendingCookie は、パスワードを確かめたユーザーとして userID を運ぶCookieを返す。
func pendingCookie(userID model.UserID) *http.Cookie {
	rec := httptest.NewRecorder()
	session.NewContinuationManager(testContinuationKey).SetTwoFactorPendingUserID(rec, userID)
	return rec.Result().Cookies()[0]
}

// uniqueRemoteAddr はテストごとに異なるIPv6の /64 の接続元を返す。
// レート制限のカウンターがコミットされて残るため、IPアドレスの単位の上限を並行するテストと分け合わない。
func uniqueRemoteAddr(t *testing.T) string {
	t.Helper()

	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("乱数の生成のエラー = %v", err)
	}
	return fmt.Sprintf("[2001:db8:%x:%x:%x::1]:12345", b[0:2], b[2:4], b[4:6])
}

// TestNew は、リカバリーコードの入力画面を表示中の言語版で描画し、戻り先をフォームとリンクへ引き継ぐことを検証する。
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
			target: "/sign_in/two_factor/recovery",
			want: []string{
				`action="/sign_in/two_factor/recovery"`,
				"リカバリーコードを入力",
				`href="/sign_in/two_factor"`,
				"ヘルプからお問い合わせ",
			},
		},
		{
			name:   "英語で戻り先あり",
			locale: i18n.LangEn,
			target: "/en/sign_in/two_factor/recovery?return_to=%2Fsettings%2Finvitation",
			want: []string{
				`action="/en/sign_in/two_factor/recovery"`,
				`name="return_to" value="/settings/invitation"`,
				`href="/en/sign_in/two_factor?return_to=%2Fsettings%2Finvitation"`,
				">Sign in<",
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
			for _, want := range append(tt.want, `content="noindex`, `autocomplete="off"`, "data-clear-on-history-restore", "data-disable-on-submit") {
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
		req := httptest.NewRequest(http.MethodGet, "/en/sign_in/two_factor/recovery?return_to=%2Fhome", nil)
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
