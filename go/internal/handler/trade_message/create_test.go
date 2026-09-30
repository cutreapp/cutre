package trade_message_test

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCreate は、交換の2人がメッセージを送り、メッセージのページの末尾へ戻ることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)

	rec := postCreate(tx, f.receiver, f.tradeID.String(), " よろしくお願いします\n平日の夜なら行けます ")

	if want := "/trades/" + f.tradeID.String() + "/messages#trade-messages-latest"; rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("応答 = %d %q、303 %s を期待", rec.Code, rec.Header().Get("Location"), want)
	}
	found := messages(t, tx, f.tradeID)
	if len(found) != 2 || found[1].SenderUserID != f.receiver.ID || found[1].Body != "よろしくお願いします\n平日の夜なら行けます" {
		t.Errorf("メッセージ = %+v、申し込まれた人の2通目を期待", found)
	}
}

// TestCreate_Invalid は、本文を受け付けなかったときは送らず、送られた本文とエラーを付けてメッセージのページを描き直すことを検証する。
func TestCreate_Invalid(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusMatched)
	tooLong := strings.Repeat("あ", 1001)

	rec := postCreate(tx, f.proposer, f.tradeID.String(), tooLong)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(), "1000文字以内で入力してください", `aria-invalid="true"`, ">"+tooLong+"</textarea>")
	if !regexp.MustCompile(`<textarea[^>]*aria-invalid="true"[^>]*autofocus`).MatchString(rec.Body.String()) {
		t.Error("本文のエラーがあるときに、入力欄へフォーカスが移らない")
	}
	if found := messages(t, tx, f.tradeID); len(found) != 1 {
		t.Errorf("メッセージの数 = %d、送らずに1通のままを期待", len(found))
	}
}

// TestCreate_RateLimited は、送信が上限を超えたときは送らず、Retry-After を付けてメッセージのページを描き直すことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)
	limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(testutil.GetTestDB()))
	for range 30 {
		if _, err := limiter.CheckTradeMessage(context.Background(), f.proposer.ID.String()); err != nil {
			t.Fatalf("レート制限の判定に失敗しました: %v", err)
		}
	}

	rec := postCreate(tx, f.proposer, f.tradeID.String(), "もう1通")

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が無い")
	}
	assertContains(t, rec.Body.String(), "メッセージを送る回数が上限に達しました。", ">もう1通</textarea>")
	if found := messages(t, tx, f.tradeID); len(found) != 1 {
		t.Errorf("メッセージの数 = %d、送らずに1通のままを期待", len(found))
	}
}

// TestCreate_Ended は、終わった交換には送らず、そのことを伝えてメッセージのページへ戻すことを検証する。
func TestCreate_Ended(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusDeclined)

	rec := postCreate(tx, f.proposer, f.tradeID.String(), "本文")

	if want := "/trades/" + f.tradeID.String() + "/messages#trade-messages-latest"; rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("応答 = %d %q、303 %s を期待", rec.Code, rec.Header().Get("Location"), want)
	}
	if !flashSet(rec) {
		t.Error("送れなかったことを伝えるメッセージが書き込まれていない")
	}
	if found := messages(t, tx, f.tradeID); len(found) != 1 {
		t.Errorf("メッセージの数 = %d、送らずに1通のままを期待", len(found))
	}
}

// TestCreate_MessageConsentRequired は、有効な同意が無いときは送らず、そのことを伝えてメッセージの利用の画面へ送ることを検証する。
func TestCreate_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)
	testutil.NewMessageConsentBuilder(t, tx, f.receiver.ID).WithWithdrawnAt(time.Now()).Build()

	rec := postCreate(tx, f.receiver, f.tradeID.String(), "本文")

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Fatalf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(rec) {
		t.Error("同意が必要なことを伝えるメッセージが書き込まれていない")
	}
	if found := messages(t, tx, f.tradeID); len(found) != 1 {
		t.Errorf("メッセージの数 = %d、送らずに1通のままを期待", len(found))
	}
}

// TestCreate_NotFound は、交換の2人以外・無い交換・読めないIDを存在しないページとして扱い、送らないことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)
	other := newUser(t, tx)
	testutil.NewMessageConsentBuilder(t, tx, other.ID).Build()

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: other, id: f.tradeID.String()},
		{name: "無い交換", user: f.proposer, id: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めないID", user: f.proposer, id: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := postCreate(tx, tt.user, tt.id, "本文"); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
	if found := messages(t, tx, f.tradeID); len(found) != 1 {
		t.Errorf("メッセージの数 = %d、送らずに1通のままを期待", len(found))
	}
}

// TestCreate_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestCreate_NoUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(nil, nil, "0199a2b0-0000-7000-8000-000000000000", "本文"); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
