package trade_decline_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getNew はユーザー user として GET /trades/{id}/decline を処理した応答を返す。
func getNew(user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id+"/decline", nil)
	rec := httptest.NewRecorder()
	newHandler().New(rec, withContext(req, user, id))

	return rec
}

// TestNew は、同意した申し込まれた人に、申し込みの内容 (渡す品・もらう品)・理由の選択肢・ひとことの欄・お断りするボタンを出すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>お断りする | Cutre</title>",
		`<h1 class="text-xl font-semibold">お断りする</h1>`,
		`href="/trades/`+tradeID.String()+`"`,
		"@"+proposer.Atname+" さんとの交換",
		"@"+proposer.Atname+" さんからの申し込み",
		"B賞・くまの子",
		"B賞・りすの子",
		`action="/trades/`+tradeID.String()+`/decline" method="post"`,
		`name="csrf_token"`,
		`id="reason" type="radio" name="reason" value="already_decided"`,
		"もう交換が決まった",
		"交換場所が合わない",
		"ほかの品と交換したい",
		"その他",
		`<textarea id="note" name="note"`,
		"理由とひとことは、メッセージで@"+proposer.Atname+" さんに届きます。",
	)
}

// TestNew_WithoutConsent は、同意の無い申し込まれた人には、ひとことの欄を出さず、ひとことに同意が要ることを添えることを検証する。
func TestNew_WithoutConsent(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, false)
	tradeID := newTrade(t, proposer, receiver)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, `name="reason"`, "ひとことを添えるには、メッセージの取り扱いへの同意が必要です。")
	if strings.Contains(body, `name="note"`) {
		t.Error("同意の無い人のお断りの画面に、ひとことの欄が含まれている")
	}
}

// TestNew_Conflict は、返事待ちでなくなった交換では、そのことを伝えて交換のページへ戻すことを検証する。
func TestNew_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newUser(t, true)
	tradeID := testutil.NewTradeBuilder(t, db, newUser(t, true).ID, receiver.ID).WithStatus(model.TradeStatusWithdrawn).Build()

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
		t.Errorf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
}

// TestNew_NotFound は、交換の2人以外・申し込んだ人・無い交換・読めないIDを存在しないページとして扱うことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver)

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: newUser(t, true), id: tradeID.String()},
		{name: "申し込んだ人", user: proposer, id: tradeID.String()},
		{name: "無い交換", user: receiver, id: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めないID", user: receiver, id: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := getNew(tt.user, tt.id); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestNew_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestNew_NoUser(t *testing.T) {
	t.Parallel()

	if rec := getNew(nil, "0199a2b0-0000-7000-8000-000000000000"); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
