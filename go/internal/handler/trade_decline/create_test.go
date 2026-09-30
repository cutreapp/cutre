package trade_decline_test

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

// postCreate はユーザー user として POST /trades/{id}/decline を処理した応答を返す。
func postCreate(user *model.User, id string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/trades/"+id+"/decline", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	newHandler().Create(rec, withContext(req, user, id))

	return rec
}

// TestCreate は、申し込まれた人が理由とひとことを添えて返事待ちの交換をお断りし、完了のメッセージを付けて交換のページへ戻すことと、
// ひとことがメッセージで届くことを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver)

	rec := postCreate(receiver, tradeID.String(), url.Values{"reason": {"already_decided"}, "note": {"先に別の方と決まってしまいました。"}})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() {
		t.Fatalf("応答 = %d %q、303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if !flashSet(rec) {
		t.Error("完了のメッセージが書き込まれていない")
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusDeclined {
		t.Errorf("交換の段階 = %q、期待値 = %q", status, model.TradeStatusDeclined)
	}
	messages, err := repository.NewTradeMessageRepository(testutil.GetTestDB()).ListByTradeID(t.Context(), tradeID)
	if err != nil || len(messages) != 1 || messages[0].Body != "先に別の方と決まってしまいました。" {
		t.Errorf("交換のメッセージ = (%+v, %v)、ひとことの1通を期待", messages, err)
	}
}

// TestCreate_Invalid は、理由を選ばなかったときに、お断りせず、ひとことを戻してエラー付きで画面を描き直すことを検証する (422)。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	proposer := newUser(t, true)
	receiver := newUser(t, true)
	tradeID := newTrade(t, proposer, receiver)

	rec := postCreate(receiver, tradeID.String(), url.Values{"note": {"ごめんなさい"}})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(),
		`href="#reason"`,
		`name="reason" value="already_decided" class="sr-only" required aria-describedby="reason-error-0" aria-invalid="true"`,
		"選んでください",
		">ごめんなさい</textarea>",
	)
	if count := strings.Count(rec.Body.String(), `aria-describedby="reason-error-0" aria-invalid="true"`); count != len(model.TradeDeclineReasons()) {
		t.Errorf("理由のエラーに関連付いたラジオ入力 = %d、期待値 = %d", count, len(model.TradeDeclineReasons()))
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusPending {
		t.Errorf("交換の段階 = %q、返事待ちのままを期待", status)
	}
}

// TestCreate_Conflict は、返事待ちでなくなった交換はお断りせず、そのことを伝えて交換のページへ戻すことを検証する。
func TestCreate_Conflict(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	receiver := newUser(t, true)
	tradeID := testutil.NewTradeBuilder(t, db, newUser(t, true).ID, receiver.ID).WithStatus(model.TradeStatusMatched).Build()

	rec := postCreate(receiver, tradeID.String(), url.Values{"reason": {"other"}})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/trades/"+tradeID.String() || !flashSet(rec) {
		t.Errorf("応答 = %d %q、メッセージを付けた 303 /trades/%s を期待", rec.Code, rec.Header().Get("Location"), tradeID)
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusMatched {
		t.Errorf("交換の段階 = %q、マッチ成立のままを期待", status)
	}
}

// TestCreate_MessageConsentRequired は、同意の無い人がひとことを添えたときは、お断りせず、そのことを伝えてメッセージの利用の画面へ送ることを検証する。
func TestCreate_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	receiver := newUser(t, false)
	tradeID := newTrade(t, newUser(t, true), receiver)

	rec := postCreate(receiver, tradeID.String(), url.Values{"reason": {"other"}, "note": {"ごめんなさい"}})

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" || !flashSet(rec) {
		t.Errorf("応答 = %d %q、メッセージを付けた 303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusPending {
		t.Errorf("交換の段階 = %q、返事待ちのままを期待", status)
	}
}

// TestCreate_NotFound は、交換の2人以外・申し込んだ人・無い交換・読めないIDを存在しないページとして扱い、お断りしないことを検証する。
func TestCreate_NotFound(t *testing.T) {
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
		if rec := postCreate(tt.user, tt.id, url.Values{"reason": {"other"}}); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
	if status := tradeStatus(t, tradeID); status != model.TradeStatusPending {
		t.Errorf("交換の段階 = %q、返事待ちのままを期待", status)
	}
}

// TestCreate_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestCreate_NoUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(nil, "0199a2b0-0000-7000-8000-000000000000", url.Values{"reason": {"other"}}); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
