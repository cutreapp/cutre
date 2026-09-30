package trade_failure_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getNew はユーザー user として GET /trades/{id}/failure を処理した応答を返す。
func getNew(user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id+"/failure", nil)
	rec := httptest.NewRecorder()
	newHandler().New(rec, withContext(req, user, id))

	return rec
}

// TestNew は、同意した交換の2人のどちらにも、記録すると交換が終わることの説明・理由の選択肢・ひとことの欄・記録するボタンを出すことを検証する。
// 相手が「交換できた」を押したあとでも開ける。
func TestNew(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver, true)

	for _, tt := range []struct{ user, partner *model.User }{{user: receiver, partner: proposer}, {user: proposer, partner: receiver}} {
		rec := getNew(tt.user, tradeID.String())

		if rec.Code != http.StatusOK {
			t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
		}
		assertContains(t, rec.Body.String(),
			"<title>交換できなかった | Cutre</title>",
			`<h1 class="text-xl font-semibold">交換できなかった</h1>`,
			`href="/trades/`+tradeID.String()+`"`,
			"@"+tt.partner.Atname+" さんとの交換を「交換できなかった」と記録します。",
			`action="/trades/`+tradeID.String()+`/failure" method="post"`,
			`name="csrf_token"`,
			`id="reason" type="radio" name="reason" value="no_show"`,
			"当日会えなかった",
			"会えたが、交換しなかった",
			"都合が悪くなった",
			"連絡が取れなくなった",
			"その他",
			`<textarea id="note" name="note"`,
			"理由とひとことは、メッセージで@"+tt.partner.Atname+" さんに届きます。",
		)
	}
}

// TestNew_WithoutConsent は、同意の無い人には、ひとことの欄を出さず、ひとことに同意が要ることを添えることを検証する。
func TestNew_WithoutConsent(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, false)
	tradeID := newTrade(t, newUser(t, true), receiver, false)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, `name="reason"`, "ひとことを添えるには、メッセージの取り扱いへの同意が必要です。")
	if strings.Contains(body, `name="note"`) {
		t.Error("同意の無い人の「交換できなかった」の画面に、ひとことの欄が含まれている")
	}
}

// TestNew_Conflict は、マッチ成立でない交換では、そのことを伝えて交換のページへ戻すことを検証する。
func TestNew_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newUser(t, true)
	for _, status := range []model.TradeStatus{model.TradeStatusPending, model.TradeStatusCompleted, model.TradeStatusCancelled} {
		tradeID := testutil.NewTradeBuilder(t, db, newUser(t, true).ID, receiver.ID).WithStatus(status).Build()

		rec := getNew(receiver, tradeID.String())

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
			t.Errorf("%s: 応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", status, rec.Code, rec.Header().Get("Location"), tradeID)
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
