package trade_history_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/handler/trade_history"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// getIndex はユーザー user として GET /trades/history をテストのトランザクション tx で処理した応答を返す。
func getIndex(tx *sql.Tx, user *model.User) *httptest.ResponseRecorder {
	db := testutil.GetTestDB()
	handler := trade_history.NewHandler(
		&config.Config{Env: "dev", Domain: "cutre.example.com"},
		usecase.NewGetTradeHistoryUsecase(
			repository.NewItemRepository(db).WithTx(tx),
			repository.NewTradeRepository(db).WithTx(tx),
			repository.NewTradeItemRepository(db).WithTx(tx),
			repository.NewUserRepository(db).WithTx(tx),
		),
	)
	req := httptest.NewRequest(http.MethodGet, "/trades/history", nil)
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
	return &model.User{ID: testutil.NewUserBuilder(t, tx).WithAtname(atname).Build(), Atname: atname, Locale: model.LocaleJa, TimeZone: "Asia/Tokyo"}
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

// TestIndex は、終わった交換だけを、終わった時刻が新しい順に、相手・終わった日と点数・段階のバッジとともに交換のページへのリンクとして並べ、
// 交換の中の画面としてパンくずとメインメニューの交換を示すことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	user := newUser(t, tx)
	haru := newUser(t, tx)
	tsumugi := newUser(t, tx)
	jst := time.FixedZone("JST", 9*60*60)
	completed := testutil.NewTradeBuilder(t, tx, haru.ID, user.ID).
		WithStatus(model.TradeStatusCompleted).
		WithEndedAt(time.Date(time.Now().Year(), 1, 18, 12, 0, 0, 0, jst)).
		Build()
	declined := testutil.NewTradeBuilder(t, tx, user.ID, tsumugi.ID).WithStatus(model.TradeStatusDeclined).Build()
	inProgress := testutil.NewTradeBuilder(t, tx, user.ID, haru.ID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()).Build()
	itemIDs := []model.ItemID{testutil.NewItemBuilder(t, tx, haru.ID, goodsID).Build(), testutil.NewItemBuilder(t, tx, user.ID, goodsID).Build()}
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), completed, itemIDs); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	rec := getIndex(tx, user)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>これまでの交換 | Cutre</title>",
		`<h1 class="text-xl font-semibold">これまでの交換</h1>`,
		`href="/trades"`,
		`href="/trades/`+completed.String()+`"`,
		"@"+haru.Atname+" さん",
		"1月18日・もらう 1点 ⇄ 渡す 1点",
		`data-variant="success">交換できた</span>`,
		`href="/trades/`+declined.String()+`"`,
		"@"+tsumugi.Atname+" さん",
		`data-variant="outline">お断り</span>`,
	)
	if strings.Contains(body, inProgress.String()) {
		t.Error("進行中の交換が含まれている")
	}
	if strings.Index(body, declined.String()) > strings.Index(body, completed.String()) {
		t.Error("終わった時刻が新しい交換を先に並べていない")
	}
	// 交換の中の画面のため、メインメニューの交換の項目を選んだ状態にする。
	if !regexp.MustCompile(`href="/trades" class="[^"]*" aria-current="true"`).MatchString(body) {
		t.Error("メインメニューの交換の項目が選んだ状態になっていない")
	}
}

// TestIndex_WithdrawnPartner は、相手が退会した交換の行で、相手を「退会したユーザー」として示すことを検証する。
func TestIndex_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	user := newUser(t, tx)
	partner := newUser(t, tx)
	testutil.NewTradeBuilder(t, tx, user.ID, partner.ID).WithStatus(model.TradeStatusCompleted).WithEndedAt(time.Now()).Build()
	testutil.WithdrawUser(t, tx, partner.ID)

	body := getIndex(tx, user).Body.String()

	assertContains(t, body, "退会したユーザー")
	if strings.Contains(body, "deleted-") {
		t.Error("匿名化したアットネームを出している")
	}
}

// TestIndex_Empty は、終わった交換が無いときにその旨を伝えることを検証する。
func TestIndex_Empty(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)

	rec := getIndex(tx, newUser(t, tx))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(), "終わった交換はまだありません")
}

// TestIndex_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestIndex_NoUser(t *testing.T) {
	t.Parallel()

	if rec := getIndex(nil, nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
