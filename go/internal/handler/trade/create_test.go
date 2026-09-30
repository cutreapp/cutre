package trade_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// form はフィクスチャーの組み合わせで申し込むフォームの値を返す。
func (f fixture) form(note string) url.Values {
	return url.Values{
		"atname":           {f.receiver.Atname},
		"receive_item_ids": {f.receiveItemID.String()},
		"give_item_ids":    {f.giveItemID.String()},
		"note":             {note},
	}
}

// flashSet は応答が完了のメッセージのCookieを書き込んだかを返す。
func flashSet(t *testing.T, header http.Header) bool {
	t.Helper()

	for _, c := range (&http.Response{Header: header}).Cookies() {
		if c.Name == session.FlashCookieName && c.Value != "" {
			return true
		}
	}
	return false
}

// hasTradeInProgress は、ユーザー userID に進行中の交換があるかを返す。
func hasTradeInProgress(t *testing.T, userID model.UserID) bool {
	t.Helper()

	exists, err := repository.NewTradeRepository(testutil.GetTestDB()).ExistsInProgressByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("進行中の交換の確認に失敗しました: %v", err)
	}

	return exists
}

// TestCreate は、交換を申し込み、完了のメッセージを付けて申し込んだ交換のページへ送ることを検証する。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t, testutil.GetTestDB(), true)

	rec := postCreate(newHandler(nil), f.proposer, f.form("はじめまして"))

	trades, err := repository.NewTradeRepository(testutil.GetTestDB()).ListInProgressByUserID(context.Background(), f.proposer.ID)
	if err != nil || len(trades) != 1 {
		t.Fatalf("進行中の交換 = (%+v, %v)、申し込んだ1件を期待", trades, err)
	}
	if want := "/trades/" + trades[0].ID.String(); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != want {
		t.Fatalf("応答 = %d %q、303 %s を期待", rec.Code, rec.Header().Get("Location"), want)
	}
	if !flashSet(t, rec.Header()) {
		t.Error("完了のメッセージが書き込まれていない")
	}
}

// TestCreate_ValidationError は、フォームを受け付けなかったときは申し込まず、送られた組み合わせとひとことを戻して
// 申し込み内容の確認の画面を描き直すことを検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	f := newFixture(t, testutil.GetTestDB(), true)
	note := strings.Repeat("あ", 1001)

	rec := postCreate(newHandler(nil), f.proposer, f.form(note))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	assertContains(t, rec.Body.String(),
		"<title>申し込み内容の確認 | Cutre</title>",
		"1000文字以内で入力してください",
		`name="receive_item_ids" value="`+f.receiveItemID.String()+`"`,
		`name="give_item_ids" value="`+f.giveItemID.String()+`"`,
		">"+note+"</textarea>",
	)
	if hasTradeInProgress(t, f.proposer.ID) {
		t.Error("受け付けなかったのに、交換を作った")
	}
}

// TestCreate_MessageConsentRequired は、有効な同意が無いときは申し込まず、そのことを伝えてメッセージの利用の画面へ送ることを検証する。
func TestCreate_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	f := newFixture(t, testutil.GetTestDB(), false)

	rec := postCreate(newHandler(nil), f.proposer, f.form(""))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/settings/message_consent" {
		t.Errorf("応答 = %d %q、303 /settings/message_consent を期待", rec.Code, rec.Header().Get("Location"))
	}
	if !flashSet(t, rec.Header()) {
		t.Error("同意が必要なことを伝えるメッセージが書き込まれていない")
	}
	if hasTradeInProgress(t, f.proposer.ID) {
		t.Error("同意が無いのに、交換を作った")
	}
}

// TestCreate_RateLimited は、申し込みが上限を超えたときは申し込まず、Retry-After を付けて申し込み内容の確認の画面を描き直すことを検証する。
func TestCreate_RateLimited(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	f := newFixture(t, db, true)
	limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(db))
	for range 10 {
		if _, err := limiter.CheckTradeProposal(context.Background(), f.proposer.ID.String()); err != nil {
			t.Fatalf("レート制限の判定に失敗しました: %v", err)
		}
	}

	rec := postCreate(newHandler(nil), f.proposer, f.form(""))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After が無い")
	}
	assertContains(t, rec.Body.String(), "申し込みの回数が上限に達しました。")
	if hasTradeInProgress(t, f.proposer.ID) {
		t.Error("上限を超えたのに、交換を作った")
	}
}

// TestCreate_NotFound は、いないユーザーへの申し込みを存在しないページとして扱うことを検証する。
func TestCreate_NotFound(t *testing.T) {
	t.Parallel()

	f := newFixture(t, testutil.GetTestDB(), true)
	form := f.form("")
	form.Set("atname", testutil.UniqueAtname())

	if rec := postCreate(newHandler(nil), f.proposer, form); rec.Code != http.StatusNotFound {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
	}
}

// TestCreate_WithoutUser は、RequireAuth を通さずに届いたリクエストで申し込まないことを検証する。
func TestCreate_WithoutUser(t *testing.T) {
	t.Parallel()

	if rec := postCreate(newHandler(nil), nil, url.Values{}); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}
