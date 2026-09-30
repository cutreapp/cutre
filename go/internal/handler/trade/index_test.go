package trade_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getIndex はユーザー user として GET /trades を処理した応答を返す。
func getIndex(tx *sql.Tx, user *model.User) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades", nil)
	rec := httptest.NewRecorder()
	newHandler(tx).Index(rec, withContext(req, user, ""))

	return rec
}

// TestIndex は、進行中の交換を相手・点数・段階のバッジとともに交換のページへのリンクとして並べ、
// マッチ候補の画面への入口と、終わった交換の段階ごとの数を添えたこれまでの交換の画面への入口を置くことと、
// メインメニューの交換の行き先として描くことを検証する。
func TestIndex(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	tradeID := testutil.NewTradeBuilder(t, tx, f.receiver.ID, f.proposer.ID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{f.giveItemID, f.receiveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).WithStatus(model.TradeStatusWithdrawn).Build()

	rec := getIndex(tx, f.proposer)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>交換 | Cutre</title>",
		`<h1 class="text-2xl font-bold">交換</h1>`,
		"進行中の交換",
		"1件",
		`href="/trades/`+tradeID.String()+`"`,
		"@"+f.receiver.Atname+" さん",
		"もらう 1点 ⇄ 渡す 1点",
		`data-variant="warning">あなたの返事待ち</span>`,
		`href="/matches"`,
		"すべて見る (0人)",
		"交換できる相手はまだいません",
		`href="/trades/history"`,
		"これまでの交換",
		"取り下げ 1",
	)
	// メインメニューの行き先のため、交換の項目に aria-current="page" を付ける。
	if !regexp.MustCompile(`href="/trades" class="[^"]*" aria-current="page"`).MatchString(body) {
		t.Error("メインメニューの交換の項目が表示中になっていない")
	}
}

// TestIndex_Empty は、進行中の交換と終わった交換が無いときにその旨を伝えることを検証する。
func TestIndex_Empty(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)

	rec := getIndex(tx, newUser(t, tx))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(), "0件", "進行中の交換はありません", `href="/trades/history"`, "まだありません")
}

// TestIndex_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestIndex_NoUser(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)

	if rec := getIndex(tx, nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
