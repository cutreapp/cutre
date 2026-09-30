package message_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/message"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// getIndex はユーザー user として GET /messages をテストのトランザクション tx で処理した応答を返す。
func getIndex(tx *sql.Tx, user *model.User) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	handler := message.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetMessagesUsecase(
			repository.NewItemRepository(db).WithTx(tx),
			repository.NewTradeRepository(db).WithTx(tx),
			repository.NewTradeItemRepository(db).WithTx(tx),
			repository.NewTradeMessageRepository(db).WithTx(tx),
			repository.NewUserRepository(db).WithTx(tx),
		),
	)
	req := httptest.NewRequest(http.MethodGet, "/messages", nil)
	ctx := i18n.SetLocale(req.Context(), i18n.LangJa)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	rec := httptest.NewRecorder()
	handler.Index(rec, req.WithContext(ctx))

	return rec
}

// newUser は日本語で使うユーザーを tx に作る。
func newUser(t *testing.T, tx *sql.Tx) *model.User {
	t.Helper()

	atname := testutil.UniqueAtname()
	return &model.User{ID: testutil.NewUserBuilder(t, tx).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa}
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

// TestIndex は、交換ごとのメッセージを、最新のメッセージが新しい順に、交換のメッセージのページへのリンクとして並べ、
// 最新の1通・未読の数・終わった交換のバッジを出すことと、メインメニューのメッセージの行き先として描くことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	user := newUser(t, tx)
	yuzu := newUser(t, tx)
	haru := newUser(t, tx)
	inProgress := testutil.NewTradeBuilder(t, tx, user.ID, yuzu.ID).Build()
	ended := testutil.NewTradeBuilder(t, tx, haru.ID, user.ID).WithStatus(model.TradeStatusCompleted).Build()
	withoutMessage := testutil.NewTradeBuilder(t, tx, user.ID, haru.ID).Build()
	testutil.NewTradeMessageBuilder(t, tx, ended, user.ID, "ありがとうございました").WithCreatedAt(time.Now().Add(-48 * time.Hour)).Build()
	testutil.NewTradeMessageBuilder(t, tx, inProgress, yuzu.ID, "東口でお願いします").Build()

	rec := getIndex(tx, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>メッセージ | Cutre</title>",
		`<h1 class="text-2xl font-bold">メッセージ</h1>`,
		"交換ごとにやり取りします。終わった交換のメッセージも読み返せます。",
		`href="/trades/`+inProgress.String()+`/messages#trade-messages-latest"`,
		"@"+yuzu.Atname+" さん",
		`font-semibold">東口でお願いします</span>`,
		`<span class="sr-only">未読 1件</span>`,
		`href="/trades/`+ended.String()+`/messages#trade-messages-latest"`,
		`href="/trades/`+withoutMessage.String()+`/messages#trade-messages-latest"`,
		"あなた: ありがとうございました",
		`data-variant="outline">終了</span>`,
		"もらう 0点 ⇄ 渡す 0点",
	)
	start := strings.Index(body, `href="/trades/`+withoutMessage.String()+`/messages#trade-messages-latest"`)
	if start < 0 {
		t.Fatal("メッセージの無い交換の行がない")
	}
	end := strings.Index(body[start:], "</a>")
	if end < 0 || !regexp.MustCompile(`<time datetime="[^"]+"[^>]*>[^<]+</time>`).MatchString(body[start:start+end]) {
		t.Error("メッセージの無い交換に申し込まれた日時を出していない")
	}
	if strings.Index(body, inProgress.String()) > strings.Index(body, ended.String()) {
		t.Error("最新のメッセージが新しい交換を先に並べていない")
	}
	// メインメニューの行き先のため、メッセージの項目に aria-current="page" を付ける。
	if !regexp.MustCompile(`href="/messages" class="[^"]*" aria-current="page"`).MatchString(body) {
		t.Error("メインメニューのメッセージの項目が表示中になっていない")
	}
}

// TestIndex_WithdrawnPartner は、相手が退会した交換の行で、相手を「退会したユーザー」として示し、
// 相手が取り消した最新のメッセージの文の主語も同じ名前にすることを検証する。
func TestIndex_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	user := newUser(t, tx)
	partner := newUser(t, tx)
	tradeID := testutil.NewTradeBuilder(t, tx, user.ID, partner.ID).WithStatus(model.TradeStatusCompleted).Build()
	testutil.NewTradeMessageBuilder(t, tx, tradeID, partner.ID, "取り消した本文").WithRetractedAt(time.Now()).Build()
	testutil.WithdrawUser(t, tx, partner.ID)

	body := getIndex(tx, user).Body.String()

	assertContains(t, body, "退会したユーザー</span>", "退会したユーザーがメッセージを取り消しました")
	if strings.Contains(body, "deleted-") || strings.Contains(body, "取り消した本文") {
		t.Error("匿名化したアットネームか、取り消した本文を出している")
	}
}

// TestIndex_Empty は、交換が無いときに、メッセージが無いことと交換を申し込めばやり取りできることを伝えることを検証する。
func TestIndex_Empty(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)

	rec := getIndex(tx, newUser(t, tx))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(), "まだメッセージはありません。交換を申し込むと、相手とメッセージでやり取りできます。")
}

// TestIndex_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestIndex_NoUser(t *testing.T) {
	t.Parallel()

	if rec := getIndex(nil, nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
