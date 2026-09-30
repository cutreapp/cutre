package trade_message_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestIndex は、進行中の交換のメッセージのページに、交換のページへの上の行・2人だけが見られることの案内・出来事とメッセージの流れ・
// ヘルプのお問い合わせへの案内・送信の欄を出し、下に固定するメインメニューを出さないことを検証する。
// 本文は改行を保ち、HTMLとして解釈させない。
func TestIndex(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)
	tradePath := "/trades/" + f.tradeID.String()

	rec := getIndex(tx, f.receiver, f.tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"<title>メッセージ | Cutre</title>",
		`<h1 class="text-xl font-semibold">@`+f.proposer.Atname+` さんとのメッセージ</h1>`,
		`href="/messages" class="block`,
		`<a href="`+tradePath+`" class="page-card`,
		`data-variant="warning">あなたの返事待ち</span>`,
		"もらう 1点 ⇄ 渡す 1点",
		"交換のページ",
		"この交換とメッセージは、あなたと@"+f.proposer.Atname+" さんだけが見られます。",
		"@"+f.proposer.Atname+" さんが申し込みました",
		"@"+f.proposer.Atname+" さん: </span>はじめまして。\n&lt;b&gt;土日&lt;/b&gt;なら動けます。</p>",
		`id="trade-messages-latest"`,
		"この交換で困ったことがあったときは",
		`action="`+tradePath+`/messages" method="post"`,
		`name="csrf_token"`,
		`name="body"`,
		`aria-label="送る"`,
		"data-sticky-composer",
		"承認・お断りは交換のページでできます。承認する前でも返信できます",
		"外の連絡先・SNSへの誘導と、お金のやり取りは禁止です",
	)
	if strings.Contains(body, `maxlength="1000"`) || strings.Contains(body, ` autofocus`) {
		t.Error("通常の入力欄に文字数のブラウザ制限またはエラー時の自動フォーカスがある")
	}
	if strings.Contains(body, "この交換は終わりました") {
		t.Error("進行中の交換に、終わったことの案内を出した")
	}
	// 送信の欄とキーボードに重ねないよう、下に固定するメインメニューを出さず、その余白も取らない。
	if !strings.Contains(body, "max-md:hidden") || strings.Contains(body, "data-main-nav") {
		t.Error("進行中の交換のメッセージで、下に固定するメインメニューを出している")
	}
	if !regexp.MustCompile(`href="/messages" class="[^"]*" aria-current="true"`).MatchString(body) {
		t.Error("メインメニューのメッセージの項目が選択中になっていない")
	}
}

// TestIndex_Guide は、送信の欄の下に、開いた人から見た交換の段階に合わせた案内を禁止事項とともに出し、
// 次にすることが無い段階では禁止事項だけを出すことを検証する。案内を出すときは、送信の欄に案内付きの目印を付ける。
func TestIndex_Guide(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	const prohibited = "外の連絡先・SNSへの誘導と、お金のやり取りは禁止です"
	pending := newFixture(t, tx, model.TradeStatusPending)
	matched := newFixture(t, tx, model.TradeStatusMatched)
	confirming := newFixture(t, tx, model.TradeStatusMatched)
	// 申し込んだ人だけが「交換できた」を押した。
	if completed, err := repository.NewTradeRepository(testutil.GetTestDB()).WithTx(tx).Complete(t.Context(), confirming.tradeID, confirming.proposer.ID); err != nil || completed == nil {
		t.Fatalf("Complete() = (%v, %v)、記録できることを期待", completed, err)
	}
	guides := []string{
		"承認される前でも、メッセージを送れます",
		"承認・お断りは交換のページでできます",
		"受け取っていたら、交換のページで「交換できた」を押してください",
	}

	tests := []struct {
		name    string
		viewer  *model.User
		tradeID model.TradeID
		want    string
	}{
		{name: "相手の返事待ち", viewer: pending.proposer, tradeID: pending.tradeID, want: guides[0]},
		{name: "マッチ成立でだれも押していない", viewer: matched.proposer, tradeID: matched.tradeID},
		{name: "あなたの確認待ち", viewer: confirming.receiver, tradeID: confirming.tradeID, want: guides[2]},
		{name: "相手の確認待ち", viewer: confirming.proposer, tradeID: confirming.tradeID},
	}
	for _, tt := range tests {
		body := getIndex(tx, tt.viewer, tt.tradeID.String()).Body.String()
		if !strings.Contains(body, prohibited) {
			t.Errorf("%s: 禁止事項を出していない", tt.name)
		}
		// 案内を出すと欄が高くなるため、フォーカスを隠さない余白を広げる目印を付ける。
		if strings.Contains(body, `data-sticky-composer="with-guide"`) != (tt.want != "") {
			t.Errorf("%s: 送信の欄の案内の目印の有無が期待と違う", tt.name)
		}
		for _, guide := range guides {
			if strings.Contains(body, guide) != (guide == tt.want) {
				t.Errorf("%s: 案内「%s」の有無が期待と違う (期待する案内: %q)", tt.name, guide, tt.want)
			}
		}
	}
}

// TestIndex_Ended は、終わった交換のメッセージのページでは送信の欄を出さずに終わったことを伝え、下のメインメニューを出すことを検証する。
func TestIndex_Ended(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusWithdrawn)

	rec := getIndex(tx, f.proposer, f.tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		"あなたが申し込みました",
		"あなた: </span>はじめまして。",
		"この交換は終わりました。メッセージは送れませんが、いつでも読み返せます。",
		"data-main-nav",
	)
	if strings.Contains(body, `name="body"`) || strings.Contains(body, "max-md:hidden") {
		t.Error("終わった交換に、送信の欄を出したか、下のメインメニューを隠した")
	}
}

// TestIndex_WithdrawnPartner は、相手が退会した交換のメッセージのページで、相手の名前・送った人・出来事の主語を
// 「退会したユーザー」にし、匿名化したアットネームを出さないことを検証する。
func TestIndex_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusWithdrawn)
	testutil.WithdrawUser(t, tx, f.proposer.ID)

	rec := getIndex(tx, f.receiver, f.tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body,
		`<h1 class="text-xl font-semibold">退会したユーザーとのメッセージ</h1>`,
		"この交換とメッセージは、あなたと退会したユーザーだけが見られます。",
		"退会したユーザーが申し込みました",
		"退会したユーザー: </span>はじめまして。",
		"この交換は終わりました。メッセージは送れませんが、いつでも読み返せます。",
	)
	if strings.Contains(body, "deleted-") {
		t.Error("匿名化したアットネームを出している")
	}
}

// TestIndex_Declined は、お断りの交換のメッセージのページで、お断りのお知らせに選んだ理由を添えることを検証する。
func TestIndex_Declined(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusDeclined)
	reason := string(model.TradeDeclineReasonPlaceMismatch)
	if _, err := repository.NewTradeEventRepository(testutil.GetTestDB()).WithTx(tx).Create(t.Context(), f.tradeID, f.receiver.ID, model.TradeEventKindDeclined, &reason); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	rec := getIndex(tx, f.proposer, f.tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	assertContains(t, rec.Body.String(), "@"+f.receiver.Atname+" さんがお断りしました", `<span class="block">理由: 交換場所が合わない</span>`)
}

// TestIndex_Retracted は、取り消したメッセージの本文を画面にもHTMLにも出さず、相手が取り消したものは相手が取り消したことを出すことを検証する。
func TestIndex_Retracted(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusMatched)
	testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.receiver.ID, "取り消した本文").WithRetractedAt(time.Now()).Build()

	rec := getIndex(tx, f.proposer, f.tradeID.String())

	body := rec.Body.String()
	assertContains(t, body, `<span class="sr-only"></span>@`+f.receiver.Atname+` さんがメッセージを取り消しました`)
	if strings.Contains(body, "取り消した本文") {
		t.Error("取り消したメッセージの本文を出した")
	}

	rec = getIndex(tx, f.receiver, f.tradeID.String())

	body = rec.Body.String()
	assertContains(t, body, "あなた: </span>メッセージを取り消しました")
	if strings.Contains(body, "取り消した本文") || strings.Contains(body, "trade-message-retract-dialog-") {
		t.Error("取り消したメッセージの本文を出したか、取り消したメッセージに取り消すボタンを出した")
	}
}

// TestIndex_Retract は、自分が送った取り消していないメッセージにだけ、取り消しの確認ダイアログを開くボタンを添えることを検証する。
// 送れない終わった交換でも、送ったメッセージは取り消せる。
func TestIndex_Retract(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusCompleted)
	mine := testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.receiver.ID, "よろしくお願いします").Build()
	testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.receiver.ID, "取り消した本文").WithRetractedAt(time.Now()).Build()
	dialogID := "trade-message-retract-dialog-" + mine.String()

	rec := getIndex(tx, f.receiver, f.tradeID.String())

	body := rec.Body.String()
	assertContains(t, body,
		`commandfor="`+dialogID+`" command="show-modal"`,
		`<dialog id="`+dialogID+`" class="alert-dialog"`,
		"メッセージを取り消しますか？",
		"取り消したメッセージは元に戻せません。@"+f.proposer.Atname+" さんの画面には「メッセージを取り消しました」と出ます。",
		`action="/trades/`+f.tradeID.String()+`/messages/`+mine.String()+`/retraction" method="post"`,
		`name="csrf_token"`,
	)
	if !regexp.MustCompile(`</span>\s*<dialog id="` + dialogID + `"`).MatchString(body) {
		t.Error("取り消しのダイアログが時刻・ボタンのspanの外にない")
	}
	// 相手のメッセージ (申し込みのひとこと) と、取り消したメッセージには添えない。
	if count := strings.Count(body, `command="show-modal"`); count != 1 {
		t.Errorf("取り消しのボタンの数 = %d、自分の取り消していない1通だけを期待", count)
	}
}

// TestIndex_Unread は、前に開いたときより後に相手が送ったメッセージの前に未読の境目を置き、開いたらそこまで読んだことにして、
// メインメニューの未読の数から読んだメッセージを除くことを検証する。
func TestIndex_Unread(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusMatched)
	readRepo := repository.NewTradeMessageReadRepository(db).WithTx(tx)
	readAt := time.Now().Add(-30 * time.Minute)
	firstMessageID := testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.proposer.ID, "前に読んだ本文").WithCreatedAt(readAt).Build()
	if err := readRepo.Save(t.Context(), f.tradeID, f.receiver.ID, model.TradeMessageReadPosition{CreatedAt: readAt, MessageID: firstMessageID}); err != nil {
		t.Fatalf("Save()のエラー = %v", err)
	}
	testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.proposer.ID, "東口でお願いします").WithCreatedAt(time.Now().Add(-10 * time.Minute)).Build()
	latestAt := time.Now().Add(-5 * time.Minute)
	latestMessageID := testutil.NewTradeMessageBuilder(t, tx, f.tradeID, f.proposer.ID, "13時にしましょう").WithCreatedAt(latestAt).Build()

	req := httptest.NewRequest(http.MethodGet, "/trades/"+f.tradeID.String()+"/messages", nil)
	req = withContext(req, f.receiver, f.tradeID.String())
	// ほかの交換の未読の1通を含めて、ミドルウェアが3通と数えた状態で開く。
	req = req.WithContext(templates.WithMainNavBadges(req.Context(), templates.MainNavBadges{UnreadMessageCount: 3}))
	rec := httptest.NewRecorder()
	newHandler(tx).Index(rec, req)

	body := rec.Body.String()
	// 境目は、読んだところより後の最初の1通の前にだけ置く。
	unread := strings.Index(body, "ここから未読")
	if unread < 0 || strings.Count(body, "ここから未読") != 1 {
		t.Fatalf("未読の境目が1つではない")
	}
	if first := strings.Index(body, "はじめまして"); first > unread || strings.Index(body, "東口でお願いします") < unread {
		t.Error("未読の境目が、読んでいた1通目と未読の1通目の間にない")
	}
	assertContains(t, body, `aria-label="メッセージ (未読 1件)"`)

	got, err := readRepo.FindLastReadPosition(t.Context(), f.tradeID, f.receiver.ID)
	if err != nil || got == nil || !got.CreatedAt.Equal(latestAt.Truncate(time.Microsecond)) || got.MessageID != latestMessageID {
		t.Errorf("FindLastReadPosition() = (%v, %v)、最新の1通の時刻 %v とID %s を期待", got, err, latestAt, latestMessageID)
	}

	// 開き直すと、もう未読の境目を出さない。
	if body := getIndex(tx, f.receiver, f.tradeID.String()).Body.String(); strings.Contains(body, "ここから未読") {
		t.Error("読んだあとに開き直しても、未読の境目を出した")
	}
}

// TestIndex_MessageConsentRequired は、有効な同意が無いときは、送信の欄の代わりにメッセージの利用の画面への案内を出し、下に固定するメインメニューは隠さないことを検証する。
func TestIndex_MessageConsentRequired(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)
	testutil.NewMessageConsentBuilder(t, tx, f.receiver.ID).WithWithdrawnAt(time.Now()).Build()

	rec := getIndex(tx, f.receiver, f.tradeID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	assertContains(t, body, "メッセージの利用への同意が必要です", "メッセージを送る前に", `href="/settings/message_consent"`)
	if strings.Contains(body, `name="body"`) {
		t.Error("同意が無いのに、送信の欄を出した")
	}
	if !strings.Contains(body, "data-main-nav") || strings.Contains(body, "max-md:hidden") {
		t.Error("送信の欄が無いのに、下に固定するメインメニューを隠した")
	}
}

// TestIndex_NotFound は、交換の2人以外・無い交換・読めないIDを存在しないページとして扱うことを検証する。
func TestIndex_NotFound(t *testing.T) {
	t.Parallel()

	_, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)

	tests := []struct {
		name string
		user *model.User
		id   string
	}{
		{name: "交換の2人以外", user: newUser(t, tx), id: f.tradeID.String()},
		{name: "無い交換", user: f.proposer, id: "0199a2b0-0000-7000-8000-000000000000"},
		{name: "読めないID", user: f.proposer, id: "not-a-uuid"},
	}
	for _, tt := range tests {
		if rec := getIndex(tx, tt.user, tt.id); rec.Code != http.StatusNotFound {
			t.Errorf("%s: ステータスコード = %d、期待値 = %d", tt.name, rec.Code, http.StatusNotFound)
		}
	}
}

// TestIndex_NoUser は、RequireAuth を通していない配線の誤りを500にすることを検証する。
func TestIndex_NoUser(t *testing.T) {
	t.Parallel()

	if rec := getIndex(nil, nil, "0199a2b0-0000-7000-8000-000000000000"); rec.Code != http.StatusInternalServerError {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestIndex_Prefetch は、ブラウザが先読みしたときは、読んだことにしないことを検証する。
func TestIndex_Prefetch(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	f := newFixture(t, tx, model.TradeStatusPending)

	req := httptest.NewRequest(http.MethodGet, "/trades/"+f.tradeID.String()+"/messages", nil)
	req.Header.Set("Sec-Purpose", "prefetch;prerender")
	rec := httptest.NewRecorder()
	newHandler(tx).Index(rec, withContext(req, f.receiver, f.tradeID.String()))

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
	}
	got, err := repository.NewTradeMessageReadRepository(db).WithTx(tx).FindLastReadPosition(t.Context(), f.tradeID, f.receiver.ID)
	if err != nil || got != nil {
		t.Errorf("FindLastReadPosition() = (%v, %v)、先読みでは記録しないことを期待", got, err)
	}
}
