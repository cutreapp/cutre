package trade_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// getShow はユーザー user として GET /trades/{id} を処理した応答を返す。
func getShow(tx *sql.Tx, user *model.User, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/trades/"+id, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx := context.WithValue(i18n.SetLocale(req.Context(), i18n.LangJa), chi.RouteCtxKey, routeCtx)
	if user != nil {
		ctx = middleware.SetUserToContext(ctx, user)
	}
	rec := httptest.NewRecorder()
	newHandler(tx).Show(rec, req.WithContext(ctx))

	return rec
}

// newTradeWithItems は、fixture の2人の返事待ちの交換を、2人の譲れるアイテムを品にして、申し込みの出来事とともに作る。
func newTradeWithItems(t *testing.T, db *sql.DB, tx *sql.Tx, f fixture) model.TradeID {
	t.Helper()

	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{f.receiveItemID, f.giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, f.proposer.ID, model.TradeEventKindProposed, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	return tradeID
}

// TestShow は、申し込んだ人が開いた返事待ちの交換のページに、段階・交換の品・取り下げの確認ダイアログ・メッセージのページへの行 (最新の1通)・これまでの流れ・
// 相手のプロフィールへの行・2人だけが見られることの案内・ヘルプのお問い合わせへの案内を出すことを検証する。
func TestShow(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	tradeID := newTradeWithItems(t, db, tx, f)
	testutil.NewTradeMessageBuilder(t, tx, tradeID, f.proposer.ID, "よろしくお願いします").Build()

	rec := getShow(tx, f.proposer, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>交換 | Cutre</title>",
		"あなた: よろしくお願いします",
		`<h1 class="text-xl font-semibold">@`+f.receiver.Atname+` さんとの交換</h1>`,
		`href="/trades" class="block`,
		`data-variant="outline">相手の返事待ち</span>`,
		"もらう (@"+f.receiver.Atname+" さんから)",
		"B賞・りすの子",
		"未開封です",
		"渡す",
		"B賞・くまの子",
		`commandfor="trade-withdraw-dialog" command="show-modal"`,
		`action="/trades/`+tradeID.String()+`/withdrawal" method="post"`,
		`name="csrf_token"`,
		`href="/trades/`+tradeID.String()+`/messages#trade-messages-latest"`,
		"@"+f.receiver.Atname+" さんとのメッセージ",
		"これまでの流れ",
		"あなたが申し込みました",
		`href="/@`+f.receiver.Atname+`"`,
		"@"+f.receiver.Atname+" さんのプロフィール",
		"この交換とメッセージは、あなたと@"+f.receiver.Atname+" さんだけが見られます。",
		"この交換で困ったことがあったときは",
		`href="https://wikino.app/s/cutre"`,
	)
	if strings.Contains(body, "/failure") || strings.Contains(body, "/cancellation") {
		t.Error("返事待ちの交換のページに、交換を終える行が含まれている")
	}
	if strings.Contains(body, `href="/trades/history"`) {
		t.Error("進行中の交換のページのパンくずに、これまでの交換が含まれている")
	}
	// 交換から辿った画面のため、メインメニューの交換の項目を選択中 (aria-current="true") にする。
	if !regexp.MustCompile(`href="/trades" class="[^"]*" aria-current="true"`).MatchString(body) {
		t.Error("メインメニューの交換の項目が選択中になっていない")
	}
}

// TestShow_Receiver は、申し込まれた人が開いたときは、申し込んだ人から見た交換の品を入れ替えて出し、取り下げを出さないことと、
// メッセージのページへの行に、相手の最新の1通と未読の数を出すことを検証する。
// 同意の無い申し込まれた人には、承認の代わりにメッセージの利用の画面への案内と、お断りの画面へのボタンを出す。
func TestShow_Receiver(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	tradeID := newTradeWithItems(t, db, tx, f)
	testutil.NewTradeMessageBuilder(t, tx, tradeID, f.proposer.ID, "よろしくお願いします").Build()

	rec := getShow(tx, f.receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">@`+f.proposer.Atname+` さんとの交換</h1>`,
		`text-muted-foreground">よろしくお願いします</span>`,
		`<span class="sr-only">未読 1件</span> <span aria-hidden="true" class="badge shrink-0" data-variant="count">1</span>`,
		`data-variant="warning">あなたの返事待ち</span>`,
		"もらう (@"+f.proposer.Atname+" さんから)",
		"@"+f.proposer.Atname+" さんが申し込みました",
		"承認すると、相手とメッセージでやり取りします。",
		`href="/settings/message_consent" class="btn w-full"`,
		`href="/trades/`+tradeID.String()+`/decline" class="btn w-full" data-variant="outline"`,
	)
	if strings.Contains(body, "trade-withdraw-dialog") {
		t.Error("申し込まれた人の交換のページに、取り下げが含まれている")
	}
	if strings.Contains(body, "/approval") {
		t.Error("同意の無い申し込まれた人の交換のページに、承認のフォームが含まれている")
	}
}

// TestShow_ReceiverWithConsent は、同意した申し込まれた人には、承認のフォームとお断りの画面へのボタンを出すことを検証する。
func TestShow_ReceiverWithConsent(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	testutil.NewMessageConsentBuilder(t, tx, f.receiver.ID).Build()
	tradeID := newTradeWithItems(t, db, tx, f)

	rec := getShow(tx, f.receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`action="/trades/`+tradeID.String()+`/approval" method="post" data-disable-on-submit`,
		`<button type="submit" class="btn w-full">承認する</button>`,
		`href="/trades/`+tradeID.String()+`/decline" class="btn w-full" data-variant="outline"`,
	)
	if strings.Contains(body, "承認すると、相手とメッセージでやり取りします。") {
		t.Error("同意した申し込まれた人の交換のページに、同意の案内が含まれている")
	}
}

// TestShow_Matched は、マッチ成立の交換のページに、次にすることの案内と「交換できた」の画面へのボタンを出し、返事の操作を出さないことを検証する。
func TestShow_Matched(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	testutil.NewMessageConsentBuilder(t, tx, f.receiver.ID).Build()
	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).WithStatus(model.TradeStatusMatched).Build()

	rec := getShow(tx, f.receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`data-variant="brand">マッチ成立</span>`,
		"メッセージで場所と日時を決めましょう",
		`href="/trades/`+tradeID.String()+`/completion" class="btn w-full">`,
		`href="/trades/`+tradeID.String()+`/failure"`,
		"交換できなかった",
		`href="/trades/`+tradeID.String()+`/cancellation"`,
		"この交換をやめる",
	)
	if strings.Contains(body, "/approval") || strings.Contains(body, "/decline") {
		t.Error("マッチ成立の交換のページに、返事の操作が含まれている")
	}
}

// TestShow_PartlyCompleted は、片方だけが「交換できた」を押した交換のページで、押していない人には確認を促して「交換できた」のボタンを出し、
// 押した人には相手を待つことを伝えてボタンを出さないことを検証する (21)。
func TestShow_PartlyCompleted(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).WithStatus(model.TradeStatusMatched).WithProposerCompletedAt(time.Now()).Build()
	completionPath := `href="/trades/` + tradeID.String() + `/completion"`

	rec := getShow(tx, f.receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		`data-variant="warning">あなたの確認待ち</span>`,
		"@"+f.proposer.Atname+" さんは「交換できた」を押しました。受け取っていたら、あなたも押してください",
		completionPath,
		`href="/trades/`+tradeID.String()+`/failure"`,
	)
	if strings.Contains(rec.Body.String(), "/cancellation") {
		t.Error("相手が「交換できた」を押した交換のページに、交換をやめる行が含まれている")
	}

	rec = getShow(tx, f.proposer, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`data-variant="outline">相手の確認待ち</span>`,
		"@"+f.receiver.Atname+" さんも「交換できた」を押すと、交換が終わります",
	)
	if strings.Contains(body, completionPath) {
		t.Error("「交換できた」を押した人の交換のページに、「交換できた」のボタンが含まれている")
	}
}

// TestShow_Declined は、お断りの交換のページを、これまでの交換の中に置き、お断りした日と人・予定だった交換の品の見出しを出し、
// これまでの流れに選んだ理由を添えることを検証する。
func TestShow_Declined(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	endedAt := time.Date(time.Now().Year(), 9, 10, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).WithStatus(model.TradeStatusDeclined).WithEndedAt(endedAt).Build()
	reason := string(model.TradeDeclineReasonPlaceMismatch)
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, f.receiver.ID, model.TradeEventKindDeclined, &reason); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	rec := getShow(tx, f.proposer, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(),
		`href="/trades/history"`,
		`data-variant="outline">お断り</span>`,
		"9月10日に@"+f.receiver.Atname+" さんがお断りしました",
		"もらう予定だった (@"+f.receiver.Atname+" さんから)",
		"渡す予定だった",
		"@"+f.receiver.Atname+" さんがお断りしました",
		"理由: 交換場所が合わない",
	)
}

// TestShow_Completed は、交換できた交換のページを、これまでの交換の中に置き、終わった日と、もらった・渡した交換の品の見出しを出し、
// 交換を進める操作を出さないことを検証する。
func TestShow_Completed(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	endedAt := time.Date(time.Now().Year(), 9, 18, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).
		WithStatus(model.TradeStatusCompleted).
		WithProposerCompletedAt(endedAt).
		WithReceiverCompletedAt(endedAt).
		WithEndedAt(endedAt).
		Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{f.receiveItemID, f.giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	rec := getShow(tx, f.receiver, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`href="/trades/history"`,
		`data-variant="success">交換できた</span>`,
		"9月18日に2人とも「交換できた」を押しました",
		"もらった (@"+f.proposer.Atname+" さんから)",
		"渡した",
	)
	for _, path := range []string{"/completion", "/failure", "/cancellation"} {
		if strings.Contains(body, `href="/trades/`+tradeID.String()+path+`"`) {
			t.Errorf("終わった交換のページに %s への入口が含まれている", path)
		}
	}
}

// TestShow_WithdrawnPartner は、相手が退会した交換のページで、相手の名前とこれまでの流れの主語を「退会したユーザー」にし、
// 匿名化したアットネームとプロフィールへのリンクを出さないことを検証する。
func TestShow_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	endedAt := time.Date(time.Now().Year(), 9, 18, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	tradeID := testutil.NewTradeBuilder(t, tx, f.proposer.ID, f.receiver.ID).
		WithStatus(model.TradeStatusCompleted).
		WithProposerCompletedAt(endedAt).
		WithReceiverCompletedAt(endedAt).
		WithEndedAt(endedAt).
		Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), tradeID, []model.ItemID{f.receiveItemID, f.giveItemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}
	if _, err := repository.NewTradeEventRepository(db).WithTx(tx).Create(t.Context(), tradeID, f.receiver.ID, model.TradeEventKindCompleted, nil); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	testutil.NewTradeMessageBuilder(t, tx, tradeID, f.receiver.ID, "ありがとうございました").Build()
	testutil.WithdrawUser(t, tx, f.receiver.ID)

	rec := getShow(tx, f.proposer, tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">退会したユーザーとの交換</h1>`,
		"もらった (退会したユーザーから)",
		"B賞・りすの子",
		"退会したユーザーが「交換できた」を押しました",
		"退会したユーザーとのメッセージ",
		"ありがとうございました",
		"この交換とメッセージは、あなたと退会したユーザーだけが見られます。",
	)
	for _, unwanted := range []string{"deleted-", "のプロフィール"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("レスポンスボディに %q が含まれている", unwanted)
		}
	}
}

// TestShow_NotFound は、交換の2人以外・無い交換・読めないIDを存在しないページとして扱うことを検証する。
func TestShow_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, true)
	tradeID := newTradeWithItems(t, db, tx, f)

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: newUser(t, tx), id: tradeID.String()},
		{name: "無い交換", user: f.proposer, id: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めないID", user: f.proposer, id: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := getShow(tx, tt.user, tt.id); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
}
