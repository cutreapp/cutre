package password_reset_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/database"
	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/handler/password_reset"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// countPasswordResetJobs は指定したユーザーへ投入されたパスワードリセットのジョブの件数を返す。
func countPasswordResetJobs(t *testing.T, userID model.UserID) int {
	t.Helper()

	var count int
	if err := testutil.GetTestDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM river_job WHERE kind = 'send_password_reset' AND args->>'user_id' = $1", userID.String(),
	).Scan(&count); err != nil {
		t.Fatalf("ジョブの件数の取得のエラー = %v", err)
	}

	return count
}

// TestCreate は、登録済みかどうかによらず、表示中の言語版の受け付けた後の画面へ同じく送ることと、
// 登録済みのアドレスのときだけジョブを投入することを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	registered := testutil.UniqueEmail("password-reset-handler-registered")
	registeredID := testutil.NewUserBuilder(t, testutil.GetTestDB()).WithEmail(registered).Build()

	tests := []struct {
		name         string
		target       string
		locale       string
		email        string
		wantLocation string
	}{
		{name: "登録済みのアドレス", target: "/password_reset", locale: i18n.LangJa, email: registered, wantLocation: "/password_reset/sent"},
		{name: "未登録のアドレス", target: "/password_reset", locale: i18n.LangJa, email: testutil.UniqueEmail("password-reset-handler-unknown"), wantLocation: "/password_reset/sent"},
		{name: "英語版", target: "/en/password_reset", locale: i18n.LangEn, email: testutil.UniqueEmail("password-reset-handler-en"), wantLocation: "/en/password_reset/sent"},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, tt.target, url.Values{"email": {tt.email}}, tt.locale, "192.0.2.52:1234"))

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tt.wantLocation {
			t.Errorf("%s: 応答 = %d %q、期待値 = 303 %q\n%s", tt.name, rec.Code, rec.Header().Get("Location"), tt.wantLocation, rec.Body.String())
		}
	}

	if got := countPasswordResetJobs(t, registeredID); got != 1 {
		t.Errorf("登録済みのユーザーへのジョブの件数 = %d、期待値 = 1", got)
	}
}

// TestCreate_Rejected は、Bot対策を通らない・形式の誤りの送信を、入力を戻したフォームの再描画で拒むことを検証する。
func TestCreate_Rejected(t *testing.T) {
	t.Parallel()

	email := testutil.UniqueEmail("password-reset-handler-rejected")

	tests := []struct {
		name         string
		turnstile    *testutil.FakeTurnstileVerifier
		email        string
		wantContains string
	}{
		{name: "Bot対策を通らない", turnstile: &testutil.FakeTurnstileVerifier{Passed: false}, email: email, wantContains: `value="` + email + `"`},
		{name: "Bot対策の検証に失敗する", turnstile: &testutil.FakeTurnstileVerifier{Err: errors.New("siteverifyの障害")}, email: email, wantContains: `value="` + email + `"`},
		{name: "アドレスとして読めない", turnstile: &testutil.FakeTurnstileVerifier{Passed: true}, email: "not-an-email", wantContains: `aria-invalid="true"`},
	}

	for _, tt := range tests {
		handler := newHandler(t, tt.turnstile)

		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {tt.email}}, i18n.LangJa, "192.0.2.53:1234"))

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusUnprocessableEntity)
		}
		assertBody(t, tt.name, rec.Body.String(), []string{tt.wantContains}, nil)
	}
}

// TestCreate_RateLimited は、同じメールアドレスへの申請が上限を超えると429とRetry-Afterで拒むことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	email := testutil.UniqueEmail("password-reset-handler-rate-limited")

	for i := range 5 {
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {email}}, i18n.LangJa, "192.0.2.54:1234"))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {email}}, i18n.LangJa, "192.0.2.54:1234"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が付いていない")
	}
}

// TestCreate_EnqueueError は、ジョブの投入に失敗しても500にせず、受け付けた後の画面へ送ることを検証する。
// 投入は登録済みのアドレスでしか行わないため、その失敗だけ応答を変えると登録の有無が分かってしまう。
func TestCreate_EnqueueError(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	closedDB, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("投入に失敗させる接続の作成のエラー = %v", err)
	}
	if err := closedDB.Close(); err != nil {
		t.Fatalf("投入に失敗させる接続のクローズのエラー = %v", err)
	}
	jobs, err := dispatcher.NewDispatcher(closedDB)
	if err != nil {
		t.Fatalf("NewDispatcher()のエラー = %v", err)
	}
	handler := password_reset.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		ratelimit.NewLimiter(repository.NewRateLimitRepository(db)),
		&testutil.FakeTurnstileVerifier{Passed: true},
		usecase.NewCreatePasswordResetUsecase(validator.NewPasswordResetCreateValidator(repository.NewUserRepository(db)), jobs),
	)
	email := testutil.UniqueEmail("password-reset-handler-enqueue-error")
	testutil.NewUserBuilder(t, db).WithEmail(email).Build()

	rec := httptest.NewRecorder()
	handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {email}}, i18n.LangJa, "192.0.2.55:1234"))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/password_reset/sent" {
		t.Errorf("応答 = %d %q、期待値 = 303 %q", rec.Code, rec.Header().Get("Location"), "/password_reset/sent")
	}
}

// TestCreate_RateLimited_Both は、IPアドレスとメールアドレスの両方が上限を超えたとき、
// Retry-Afterを両方の制限が解除される時刻までの秒数にすることを検証する。
func TestCreate_RateLimited_Both(t *testing.T) {
	t.Parallel()

	handler := newHandler(t, &testutil.FakeTurnstileVerifier{Passed: true})
	email := testutil.UniqueEmail("password-reset-handler-rate-limited-both")
	const remoteAddr = "192.0.2.56:1234"

	// 同じアドレスで5回、別々のアドレスで15回送り、IPアドレスとメールアドレスの両方を上限まで数える。
	for i := range 20 {
		target := email
		if i >= 5 {
			target = testutil.UniqueEmail("password-reset-handler-rate-limited-other")
		}
		rec := httptest.NewRecorder()
		handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {target}}, i18n.LangJa, remoteAddr))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%d回目のステータスコード = %d、期待値 = %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, newRequest(http.MethodPost, "/password_reset", url.Values{"email": {email}}, i18n.LangJa, remoteAddr))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q、秒数を期待", rec.Header().Get("Retry-After"))
	}
	// どちらの制限も1時間の枠で数えるため、解除されるのは次の枠の始まり。
	want := int(math.Ceil(time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)).Seconds()))
	if retryAfter < want-1 || retryAfter > want+1 {
		t.Errorf("Retry-After = %d、期待値 = %d (前後1秒を許容)", retryAfter, want)
	}
}
