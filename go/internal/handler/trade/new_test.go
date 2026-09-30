package trade_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getNew はユーザー user としてアットネーム atname の GET /@{atname}/trades/new を処理した応答を返す。
func getNew(tx *sql.Tx, user *model.User, atname, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/@"+atname+"/trades/new"+query, nil)
	rec := httptest.NewRecorder()
	newHandler(tx).New(rec, withContext(req, user, atname))

	return rec
}

// TestNew は、交換できるアイテムを、もらうもの・渡すものの選択肢として組み合わせの確認へGETで送るフォームに並べ、
// 交換の中の画面として交換・マッチ候補・相手のプロフィールへのパンくずを出すことを検証する。
// クエリで選んだアイテムは選んだ状態で出す。
func TestNew(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)

	rec := getNew(tx, f.proposer, f.receiver.Atname, "?receive_item_ids="+f.receiveItemID.String()+"&receive_item_ids=not-a-uuid")

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>組み合わせを選ぶ | Cutre</title>",
		`<h1 class="text-xl font-semibold">組み合わせを選ぶ</h1>`,
		`href="/trades" class="block`,
		`href="/matches" class="block`,
		`href="/@`+f.receiver.Atname+`" class="block`,
		"受けるかどうかは @"+f.receiver.Atname+" さんが決めます",
		`action="/@`+f.receiver.Atname+`/trades/new/confirmation" method="get"`,
		`name="receive_item_ids" value="`+f.receiveItemID.String()+`" checked`,
		`name="give_item_ids" value="`+f.giveItemID.String()+`">`,
		"B賞・りすの子",
		"未開封です",
		"B賞・くまの子",
	)
	// 交換から辿った画面のため、メインメニューの交換の項目を選択中 (aria-current="true") にする。
	if !regexp.MustCompile(`href="/trades" class="[^"]*" aria-current="true"`).MatchString(body) {
		t.Error("メインメニューの交換の項目が選択中になっていない")
	}
}

// TestNew_MessageConsentRequired は、有効な同意が無いときは、選ぶフォームの代わりにメッセージの利用の画面への案内を出すことを検証する。
func TestNew_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, false)

	rec := getNew(tx, f.proposer, f.receiver.Atname, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "メッセージの利用への同意が必要です", `href="/settings/message_consent" class="btn"`)
	if strings.Contains(body, "/trades/new/confirmation") {
		t.Error("同意が無いのに、組み合わせを選ぶフォームを出した")
	}
}

// TestNew_NotTradable は、もらえるものか渡せるものが無いときは、申し込めないことを伝えてリストへ案内することを検証する。
func TestNew_NotTradable(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	proposer := newUser(t, tx)
	testutil.NewMessageConsentBuilder(t, tx, proposer.ID).Build()
	receiver := newUser(t, tx)

	rec := getNew(tx, proposer, receiver.Atname, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "いまは @"+receiver.Atname+" さんと交換できる組み合わせがありません。", `href="/list" class="btn"`)
	if strings.Contains(body, "/trades/new/confirmation") {
		t.Error("申し込めないのに、組み合わせを選ぶフォームを出した")
	}
}

// TestNew_NotFound は、いないユーザーと自分のアットネームを存在しないページとして扱うことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	proposer := newUser(t, tx)

	for name, atname := range map[string]string{"いない": testutil.UniqueAtname(), "自分自身": proposer.Atname} {
		if rec := getNew(tx, proposer, atname, ""); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestNew_WithoutUser は、RequireAuth を通さずに届いたリクエストで描画しないことを検証する。
func TestNew_WithoutUser(t *testing.T) {
	t.Parallel()

	if rec := getNew(nil, nil, "yuzu", ""); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
