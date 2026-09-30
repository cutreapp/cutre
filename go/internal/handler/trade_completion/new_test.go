package trade_completion_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getNew はユーザー user として GET /trades/{id}/completion を処理した応答を返す。
func getNew(user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id+"/completion", nil)
	rec := httptest.NewRecorder()
	newHandler().New(rec, withContext(req, user, id))

	return rec
}

// TestNew は、同意した人に、交換の品 (受け取った・渡した)・ひとことの欄・相手を待つ案内・記録するボタンを出すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver, false)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		"<title>交換できた | Cutre</title>",
		`<h1 class="text-xl font-semibold">交換できた</h1>`,
		`href="/trades/`+tradeID.String()+`"`,
		"@"+proposer.Atname+" さんとの交換",
		"受け取った (@"+proposer.Atname+" さんから)",
		"渡した",
		"B賞・くまの子",
		"B賞・りすの子",
		`action="/trades/`+tradeID.String()+`/completion" method="post"`,
		`name="csrf_token"`,
		`<textarea id="note" name="note" rows="3" maxlength="1000" aria-describedby="note-hint">`,
		"メッセージで@"+proposer.Atname+" さんに届きます。",
		"@"+proposer.Atname+" さんも「交換できた」を押すと、交換が終わります。",
		"交換できたと記録する",
	)
}

// TestNew_PartnerCompleted は、相手がすでに押していれば、記録すると交換が終わることを案内することを検証する。
func TestNew_PartnerCompleted(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver, true)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(), "@"+proposer.Atname+" さんは「交換できた」を押しています。記録すると交換が終わり")
}

// TestNew_EnglishQuantityNotice は、英語の画面で、交換を終えると数量が1点ずつ減り、0点になった品だけがリストから外れると案内することを検証する。
func TestNew_EnglishQuantityNotice(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	for _, partnerCompleted := range []bool{false, true} {
		tradeID := newTrade(t, proposer, receiver, partnerCompleted)
		req := httptest.NewRequest(http.MethodGet, "/trades/"+tradeID.String()+"/completion", nil)
		req = withContext(req, receiver, tradeID.String())
		req = req.WithContext(i18n.SetLocale(req.Context(), i18n.LangEn))
		rec := httptest.NewRecorder()
		newHandler().New(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("相手の記録済み = %t: ステータスコード = %d、期待値 = %d", partnerCompleted, rec.Code, http.StatusOK)
		}
		assertContains(t, rec.Body.String(), "decrease by one. Items with none left will be removed from your lists.")
	}
}

// TestNew_WithoutConsent は、同意の無い人には、ひとことの欄を出さないことを検証する。
func TestNew_WithoutConsent(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, false)
	tradeID := newTrade(t, proposer, receiver, false)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "交換できたと記録する")
	if strings.Contains(body, `name="note"`) {
		t.Error("同意の無い人の「交換できた」の画面に、ひとことの欄が含まれている")
	}
}

// TestNew_Conflict は、マッチ成立でない交換と、すでに押した交換では、そのことを伝えて交換のページへ戻すことを検証する。
func TestNew_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	proposer := newUser(t, true)
	receiver := newUser(t, true)
	pending := testutil.NewTradeBuilder(t, db, proposer.ID, receiver.ID).Build()
	pressed := newTrade(t, proposer, receiver, true)

	for _, tt := range []struct {
		name    string
		user    *model.User
		tradeID model.TradeID
	}{
		{name: "返事待ちの交換", user: receiver, tradeID: pending},
		{name: "すでに押した交換", user: proposer, tradeID: pressed},
	} {
		rec := getNew(tt.user, tt.tradeID.String())
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tt.tradeID.String() || !flashSet(rec) {
			t.Errorf("%s: 応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", tt.name, rec.Code, rec.Header().Get("Location"), tt.tradeID)
		}
	}
}

// TestNew_NotFound は、交換の2人以外・無い交換・読めないIDを存在しないページとして扱うことを検証する。
func TestNew_NotFound(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, true)
	tradeID := newTrade(t, newUser(t, true), receiver, false)

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: newUser(t, true), id: tradeID.String()},
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
