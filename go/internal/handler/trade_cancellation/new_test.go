package trade_cancellation_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getNew はユーザー user として GET /trades/{id}/cancellation を処理した応答を返す。
func getNew(user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id+"/cancellation", nil)
	rec := httptest.NewRecorder()
	newHandler().New(rec, withContext(req, user, id))

	return rec
}

// TestNew は、同意した交換の2人のどちらにも、元に戻せないことの説明・理由の選択肢・必須のひとことの欄・やめるボタンを出すことを検証する。
func TestNew(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver, false)

	for _, tt := range []struct{ user, partner *model.User }{{user: receiver, partner: proposer}, {user: proposer, partner: receiver}} {
		rec := getNew(tt.user, tradeID.String())

		if rec.Code != http.StatusOK {
			t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
		}
		assertContains(t, rec.Body.String(),
			"<title>この交換をやめる | Cutre</title>",
			`<h1 class="text-xl font-semibold">この交換をやめる</h1>`,
			`href="/trades/`+tradeID.String()+`"`,
			"@"+tt.partner.Atname+" さんとの交換をやめます。やめたあとは元に戻せません。",
			`action="/trades/`+tradeID.String()+`/cancellation" method="post"`,
			`name="csrf_token"`,
			`id="reason" type="radio" name="reason" value="decided_elsewhere"`,
			"ほかの人と交換が決まった",
			"予定が合わなくなった",
			"品物が手元になくなった",
			"その他",
			`<textarea id="note" name="note" rows="3" maxlength="1000" required aria-describedby="note-hint"`,
			"理由とひとことは、メッセージで@"+tt.partner.Atname+" さんに届きます。",
			`<button type="submit" class="btn w-full" data-variant="destructive">交換をやめる</button>`,
		)
	}
}

// TestNew_WithoutConsent は、同意の無い人には、ひとことを届けられないため、フォームの代わりにメッセージの利用の画面へ案内することを検証する。
func TestNew_WithoutConsent(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, false)
	tradeID := newTrade(t, newUser(t, true), receiver, false)

	rec := getNew(receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "交換をやめるときは、ひとことをメッセージで相手に届けます。", `href="/settings/message_consent"`)
	if strings.Contains(body, `action="/trades/`+tradeID.String()+`/cancellation"`) {
		t.Error("同意の無い人の交換をやめる画面に、やめるフォームが含まれている")
	}
}

// TestNew_Conflict は、マッチ成立でない交換と、どちらかが「交換できた」を押した交換では、そのことを伝えて交換のページへ戻すことを検証する。
func TestNew_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newUser(t, true)
	tradeIDs := []model.TradeID{
		testutil.NewTradeBuilder(t, db, newUser(t, true).ID, receiver.ID).Build(),
		testutil.NewTradeBuilder(t, db, newUser(t, true).ID, receiver.ID).WithStatus(model.TradeStatusFailed).Build(),
		newTrade(t, newUser(t, true), receiver, true),
	}
	for _, tradeID := range tradeIDs {
		rec := getNew(receiver, tradeID.String())

		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
			t.Errorf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
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
