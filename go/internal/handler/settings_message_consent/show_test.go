package settings_message_consent_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/settings_message_consent"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newHandler はテスト用のトランザクションの中で動く Handler と、そのトランザクションを返す。
// 同意の表示と同意のUseCaseは自分でトランザクションを開かないため、テストのトランザクションで包める。
// 同意をやめるUseCaseは自分でトランザクションを開くため、コミットした行を読み書きする (TestDelete* は newCommittedUser を使う)。
func newHandler(t *testing.T) (*settings_message_consent.Handler, *sql.Tx) {
	t.Helper()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewMessageConsentRepository(db).WithTx(tx)

	return settings_message_consent.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		session.NewFlashManager(),
		usecase.NewGetMessageConsentUsecase(repo),
		usecase.NewCreateMessageConsentUsecase(repo),
		usecase.NewWithdrawMessageConsentUsecase(
			db,
			validator.NewMessageConsentWithdrawValidator(repository.NewTradeRepository(db)),
			repository.NewMessageConsentRepository(db),
			repository.NewUserRepository(db),
		),
	), tx
}

// newUser はテスト用のユーザーを作り、ログイン中のユーザーとしてcontextへ載せる値を返す。
func newUser(t *testing.T, tx *sql.Tx) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, tx).Build()
	user, err := repository.NewUserRepository(testutil.GetTestDB()).WithTx(tx).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// withUser はログイン中のユーザーとロケールを載せたリクエストを返す。ロケールは本番では UserLocale が users.locale から決める。
// user がnilのときは RequireAuth を通していないリクエストとして、ユーザーを載せない。
func withUser(req *http.Request, user *model.User) *http.Request {
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}

	return req.WithContext(ctx)
}

// show はログイン中のユーザーとしてメッセージの利用の画面を開く。
func show(handler *settings_message_consent.Handler, user *model.User) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.Show(rec, withUser(httptest.NewRequest(http.MethodGet, "/settings/message_consent", nil), user))

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

// TestShow_Agreed は、有効な同意があれば、同意した日 (ユーザーのタイムゾーンの日付) と同意した内容、
// 同意をやめるフォーム (DELETEに上書きしたPOST) を描画し、同意するフォームを出さないことを検証する。
func TestShow_Agreed(t *testing.T) {
	t.Parallel()

	handler, tx := newHandler(t)
	user := newUser(t, tx)
	tokyo, _ := time.LoadLocation(model.DefaultTimeZone)
	// 東京では1月2日、UTCでは1月1日にあたる時刻。
	agreedAt := time.Date(time.Now().In(tokyo).Year(), 1, 2, 0, 30, 0, 0, tokyo)
	testutil.NewMessageConsentBuilder(t, tx, user.ID).WithAgreedAt(agreedAt).Build()

	rec := show(handler, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	// マイページから辿った画面のため、メインメニューのマイページの項目を選択中 (aria-current="true") にする。
	myPageLink := regexp.MustCompile(`href="/@` + regexp.QuoteMeta(user.Atname) + `" class="[^"]*" aria-current="true"`)
	if !myPageLink.MatchString(body) {
		t.Error("メインメニューのマイページの項目に aria-current=\"true\" が付いていない")
	}
	assertContains(t, body,
		"<title>メッセージの利用 | Cutre</title>",
		`<meta name="robots" content="noindex">`,
		`<nav aria-label="パンくずリスト">`,
		`<h1 class="text-xl font-semibold">メッセージの利用</h1>`,
		`<span class="badge" data-variant="brand">同意済み</span>`,
		"1月2日に同意しました",
		"同意した内容",
		"メッセージは、あなたと交換の相手だけが見られます",
		`action="/settings/message_consent" method="post"`,
		`<input type="hidden" name="_method" value="DELETE">`,
		"同意をやめる",
	)
	assertNotContains(t, body, "未同意", ">同意する<")
}

// TestShow_NotAgreed は、有効な同意が無いとき (記録が無い・やめた・古い版) に、今の文面と同意するフォームを描画し、
// 同意をやめるフォームを出さないことを検証する。
func TestShow_NotAgreed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(builder *testutil.MessageConsentBuilder)
	}{
		{name: "記録が無い", setup: func(*testutil.MessageConsentBuilder) {}},
		{name: "やめた", setup: func(builder *testutil.MessageConsentBuilder) { builder.WithWithdrawnAt(time.Now()).Build() }},
		{name: "古い版", setup: func(builder *testutil.MessageConsentBuilder) {
			builder.WithVersion(model.CurrentMessageConsentVersion - 1).Build()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, tx := newHandler(t)
			user := newUser(t, tx)
			tt.setup(testutil.NewMessageConsentBuilder(t, tx, user.ID))

			rec := show(handler, user)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			body := rec.Body.String()
			assertContains(t, body,
				"交換の申し込みと承認には、メッセージの取り扱いへの同意が必要です。",
				`<span class="badge" data-variant="outline">未同意</span>`,
				"同意する内容",
				"メッセージは、あなたと交換の相手だけが見られます",
				`action="/settings/message_consent" method="post"`,
				">同意する<",
			)
			assertNotContains(t, body, "同意済み", `name="_method" value="DELETE"`)
		})
	}
}

// TestShow_WithoutUser は、RequireAuth を通さずに届いたリクエストを誰かの同意として描画しないことを検証する。
func TestShow_WithoutUser(t *testing.T) {
	t.Parallel()

	handler, _ := newHandler(t)
	rec := show(handler, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
