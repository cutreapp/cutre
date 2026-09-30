package trade_completion_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// postCreate はユーザー user として POST /trades/{id}/completion を処理した応答を返す。
func postCreate(user *model.User, id string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/trades/"+id+"/completion", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler().Create(rec, withContext(req, user, id))

	return rec
}

// TestCreate は、片方がひとことを添えて押すと、マッチ成立のまま押したことを記録して交換のページへ戻し、ひとことがメッセージで届くことと、
// もう片方も押すと交換が終わることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver, false)

	rec := postCreate(receiver, tradeID.String(), url.Values{"note": {"ありがとうございました。"}})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
		t.Fatalf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if trade := findTrade(t, tradeID); trade.Status != model.TradeStatusMatched || trade.ReceiverCompletedAt == nil {
		t.Errorf("片方が押したあとの交換 = %+v、申し込まれた人の時刻が入ったマッチ成立を期待", trade)
	}
	messages, err := repository.NewTradeMessageRepository(testutil.GetTestDB()).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].Body != "ありがとうございました。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、ひとことの1通を期待", messages, err)
	}

	rec = postCreate(proposer, tradeID.String(), url.Values{})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
		t.Fatalf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if trade := findTrade(t, tradeID); trade.Status != model.TradeStatusCompleted {
		t.Errorf("交換の段階 = %q、期待値 = %q", trade.Status, model.TradeStatusCompleted)
	}
}

// TestCreate_Invalid は、長すぎるひとことを受け付けず、ひとことを戻してエラー付きで画面を描き直すことを検証する (422)。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, true)
	tradeID := newTrade(t, newUser(t, true), receiver, false)
	note := strings.Repeat("あ", 1001)

	rec := postCreate(receiver, tradeID.String(), url.Values{"note": {note}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(),
		`href="#note"`,
		`aria-describedby="note-hint note-error-0" aria-invalid="true"`,
		"1000文字以内で入力してください",
		">"+note+"</textarea>",
	)
	if trade := findTrade(t, tradeID); trade.ReceiverCompletedAt != nil {
		t.Errorf("交換 = %+v、押していないままを期待", trade)
	}
}

// TestCreate_Conflict は、押せない交換 (すでに押した) では記録せず、そのことを伝えて交換のページへ戻すことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	tradeID := newTrade(t, proposer, newUser(t, true), true)

	rec := postCreate(proposer, tradeID.String(), url.Values{})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
		t.Errorf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if trade := findTrade(t, tradeID); trade.Status != model.TradeStatusMatched {
		t.Errorf("交換の段階 = %q、マッチ成立のままを期待", trade.Status)
	}
}

// TestCreate_MessageConsentRequired は、同意の無い人がひとことを添えたときは、記録せず、そのことを伝えてメッセージの利用の画面へ送ることを検証する。
func TestCreate_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, false)
	tradeID := newTrade(t, newUser(t, true), receiver, false)

	rec := postCreate(receiver, tradeID.String(), url.Values{"note": {"ありがとうございました"}})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" || !flashSet(rec) {
		t.Errorf("応答 = %d %q、メッセージを付けた 303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if trade := findTrade(t, tradeID); trade.ReceiverCompletedAt != nil {
		t.Errorf("交換 = %+v、押していないままを期待", trade)
	}
}

// TestCreate_NotFound は、交換の2人以外・無い交換・読めないIDを存在しないページとして扱い、記録しないことを検証する。
func TestCreate_NotFound(t *testing.T) {
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
		if rec := postCreate(tt.user, tt.id, url.Values{}); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
	if trade := findTrade(t, tradeID); trade.ProposerCompletedAt != nil || trade.ReceiverCompletedAt != nil {
		t.Errorf("交換 = %+v、だれも押していないままを期待", trade)
	}
}

// TestCreate_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestCreate_NoUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(nil, "0199a2b0-0000-7000-8000-000000000000", url.Values{}); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
