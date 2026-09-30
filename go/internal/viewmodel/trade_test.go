package viewmodel_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewTradeProposal は、交換できるアイテムをカテゴリーとグッズの名前・ひとことを添えた選択肢にし、
// 選んだIDのアイテムだけを選んでいる状態にすることを検証する。
func TestNewTradeProposal(t *testing.T) {
	t.Parallel()

	category := &model.EventCategory{ID: model.EventCategoryID(uuid.New()), Name: "B賞 ラバーマスコット"}
	goods := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: category.ID, Name: "りすの子"}
	item := func(note string) *model.Item {
		return &model.Item{ID: model.ItemID(uuid.New()), GoodsID: goods.ID, Note: note}
	}
	receivableA, receivableB, givable := item("未開封です"), item(""), item("")
	match := model.Match{
		Receivable: []model.MatchItem{{Item: receivableA, Quantity: 1}, {Item: receivableB, Quantity: 2}},
		Givable:    []model.MatchItem{{Item: givable, Quantity: 1}},
	}

	proposal := viewmodel.NewTradeProposal(
		"yuzu",
		match,
		map[model.GoodsID]*model.Goods{goods.ID: goods},
		map[model.EventCategoryID]*model.EventCategory{category.ID: category},
		[]model.ItemID{receivableB.ID},
		nil,
	)

	if proposal.Atname != "yuzu" || len(proposal.Receivable) != 2 || len(proposal.Givable) != 1 {
		t.Fatalf("NewTradeProposal() = %+v、相手とアイテムの2つ・1つの選択肢を期待", proposal)
	}
	want := viewmodel.TradeItemChoice{
		ID:           receivableA.ID.String(),
		TradeItemRow: viewmodel.TradeItemRow{EventCategoryName: "B賞 ラバーマスコット", GoodsName: "りすの子", Note: "未開封です"},
	}
	if proposal.Receivable[0] != want {
		t.Errorf("Receivable[0] = %+v、%+v を期待", proposal.Receivable[0], want)
	}
	if got := proposal.ReceiveItemIDs(); len(got) != 1 || got[0] != receivableB.ID.String() {
		t.Errorf("ReceiveItemIDs() = %v、選んだ %s だけを期待", got, receivableB.ID)
	}
	if got := proposal.GiveItemIDs(); len(got) != 0 {
		t.Errorf("GiveItemIDs() = %v、空を期待", got)
	}
}

// TestNewTradeRows は、交換を開いたユーザーから見た相手・もらう点数・渡す点数・段階のバッジの行にすることを検証する。
func TestNewTradeRows(t *testing.T) {
	t.Parallel()

	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "yuzu"}
	received := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: partner.ID, ReceiverUserID: me, Status: model.TradeStatusPending}
	proposed := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched}
	items := map[model.TradeID][]*model.Item{
		received.ID: {{UserID: partner.ID}, {UserID: partner.ID}, {UserID: me}},
	}

	rows := viewmodel.NewTradeRows(i18n.SetLocale(context.Background(), i18n.LangJa), me, []*model.Trade{received, proposed}, map[model.UserID]*model.User{partner.ID: partner}, items)

	want := []viewmodel.TradeRow{
		{
			ID: received.ID.String(), PartnerName: "@yuzu さん", ReceiveQuantity: 2, GiveQuantity: 1,
			Status: viewmodel.TradeStatusBadge{LabelKey: "trade_status_awaiting_your_reply", Variant: "warning"},
		},
		{
			ID: proposed.ID.String(), PartnerName: "@yuzu さん",
			Status: viewmodel.TradeStatusBadge{LabelKey: "trade_status_matched", Variant: "brand"},
		},
	}
	if len(rows) != len(want) {
		t.Fatalf("行の数 = %d、期待値 = %d", len(rows), len(want))
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("rows[%d] = %+v、%+v を期待", i, rows[i], want[i])
		}
	}
}

// TestNewTradeRows_WithdrawnPartner は、退会した相手を、匿名化したアットネームではなく退会したユーザーとして
// 表示言語の名前で示すことを検証する。
func TestNewTradeRows_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	me := model.UserID(uuid.New())
	withdrawn := &model.User{ID: model.UserID(uuid.New()), Atname: "deleted-x", DeletedAt: new(time.Now())}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: withdrawn.ID, Status: model.TradeStatusCompleted}
	partners := map[model.UserID]*model.User{withdrawn.ID: withdrawn}

	for _, tt := range []struct {
		lang string
		want string
	}{
		{lang: i18n.LangJa, want: "退会したユーザー"},
		{lang: i18n.LangEn, want: "Withdrawn user"},
	} {
		rows := viewmodel.NewTradeRows(i18n.SetLocale(context.Background(), tt.lang), me, []*model.Trade{trade}, partners, nil)
		if len(rows) != 1 || rows[0].PartnerName != tt.want {
			t.Errorf("%s: NewTradeRows() = %+v、相手の名前 %q を期待", tt.lang, rows, tt.want)
		}
	}
}

// TestNewTradeDetail は、交換の品を開いたユーザーから見たもらう品・渡す品に分け、これまでの流れを誰が起こしたかで文を選び、
// 選んだ理由を添えることと、取り下げを申し込んだ人の返事待ちのときだけ許すことを検証する。
func TestNewTradeDetail(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "rin"}
	category := &model.EventCategory{ID: model.EventCategoryID(uuid.New()), Name: "B賞"}
	goods := &model.Goods{ID: model.GoodsID(uuid.New()), EventCategoryID: category.ID, Name: "ねこの子"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusPending}
	items := []*model.Item{{UserID: partner.ID, GoodsID: goods.ID, Note: "未開封"}, {UserID: me, GoodsID: goods.ID}}
	reason := string(model.TradeDeclineReasonPlaceMismatch)
	events := []*model.TradeEvent{
		// 日本時間では9月23日の朝になる。
		{ActorUserID: me, Kind: model.TradeEventKindProposed, CreatedAt: time.Date(2026, 9, 22, 23, 0, 0, 0, time.UTC)},
		{ActorUserID: partner.ID, Kind: model.TradeEventKindDeclined, Reason: &reason, CreatedAt: time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC)},
	}
	goodsMap := map[model.GoodsID]*model.Goods{goods.ID: goods}
	categories := map[model.EventCategoryID]*model.EventCategory{category.ID: category}

	latest := &model.TradeMessage{SenderUserID: partner.ID, Body: "よろしくお願いします", CreatedAt: time.Date(2026, 9, 29, 1, 5, 0, 0, time.UTC)}

	detail := viewmodel.NewTradeDetail(ctx, me, trade, partner, items, goodsMap, categories, events, latest, 2, true, loc, now)

	if detail.ID != trade.ID.String() || detail.PartnerName != "@rin さん" || detail.PartnerAtname != "rin" || !detail.CanWithdraw || detail.CanReply || !detail.MessageConsentValid || detail.StatusNoteKey != "" {
		t.Errorf("NewTradeDetail() = %+v、相手が rin の、返事はできず取り下げられる交換を期待", detail)
	}
	if detail.Status != (viewmodel.TradeStatusBadge{LabelKey: "trade_status_awaiting_partner_reply", Variant: "outline"}) {
		t.Errorf("Status = %+v、相手の返事待ちを期待", detail.Status)
	}
	wantReceive := viewmodel.TradeItemRow{EventCategoryName: "B賞", GoodsName: "ねこの子", Note: "未開封"}
	if len(detail.Receive) != 1 || detail.Receive[0] != wantReceive || len(detail.Give) != 1 {
		t.Errorf("Receive = %+v, Give = %+v、もらう品と渡す品の1点ずつを期待", detail.Receive, detail.Give)
	}
	wantEvents := []viewmodel.TradeEventRow{
		{Date: "9月23日", TextKey: "trade_event_proposed_self"},
		{Date: "9月24日", TextKey: "trade_event_declined_partner", ActorName: "@rin さん", ReasonKey: "trade_decline_reason_place_mismatch"},
	}
	if len(detail.Events) != 2 || detail.Events[0] != wantEvents[0] || detail.Events[1] != wantEvents[1] {
		t.Errorf("Events = %+v、%+v を期待", detail.Events, wantEvents)
	}
	wantLatest := viewmodel.TradeMessagePreview{Body: "よろしくお願いします", SentAt: "10:05", DateTime: "2026-09-29T10:05:00+09:00"}
	if detail.LatestMessage == nil || *detail.LatestMessage != wantLatest || detail.UnreadMessageCount != 2 {
		t.Errorf("LatestMessage = %+v, UnreadMessageCount = %d、%+v と未読の2通を期待", detail.LatestMessage, detail.UnreadMessageCount, wantLatest)
	}

	for _, tt := range []struct {
		name          string
		viewer        model.UserID
		status        model.TradeStatus
		wantReply     bool
		wantStatusKey string
	}{
		{name: "申し込まれた人", viewer: partner.ID, status: model.TradeStatusPending, wantReply: true},
		{name: "マッチ成立", viewer: me, status: model.TradeStatusMatched, wantStatusKey: "trade_show_status_note_matched"},
		{name: "マッチ成立を申し込まれた人が開く", viewer: partner.ID, status: model.TradeStatusMatched, wantStatusKey: "trade_show_status_note_matched"},
		{name: "お断りを申し込まれた人が開く", viewer: partner.ID, status: model.TradeStatusDeclined},
	} {
		other := *trade
		other.Status = tt.status
		got := viewmodel.NewTradeDetail(ctx, tt.viewer, &other, partner, nil, nil, nil, nil, nil, 0, false, loc, now)
		if got.CanWithdraw || got.LatestMessage != nil || got.CanReply != tt.wantReply || got.StatusNoteKey != tt.wantStatusKey {
			t.Errorf("%s: CanWithdraw = %v, LatestMessage = %+v, CanReply = %v, StatusNoteKey = %q、取り下げられず、メッセージが無く、返事 %v・案内 %q を期待",
				tt.name, got.CanWithdraw, got.LatestMessage, got.CanReply, got.StatusNoteKey, tt.wantReply, tt.wantStatusKey)
		}
	}
}

// TestNewTradeDetail_Completion は、マッチ成立の交換で、2人のどちらが「交換できた」を押したかに合わせて、
// 押せるか・段階のバッジ・次にすることの案内を選ぶことを検証する。
func TestNewTradeDetail_Completion(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "kanade"}
	pressedAt := now.Add(-time.Hour)

	tests := []struct {
		name                 string
		trade                model.Trade
		wantComplete         bool
		wantPartnerCompleted bool
		wantStatus           viewmodel.TradeStatusBadge
		wantStatusKey        string
	}{
		{
			name:          "だれも押していない",
			trade:         model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched},
			wantComplete:  true,
			wantStatus:    viewmodel.TradeStatusBadge{LabelKey: "trade_status_matched", Variant: "brand"},
			wantStatusKey: "trade_show_status_note_matched",
		},
		{
			name:          "自分だけが押した",
			trade:         model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched, ProposerCompletedAt: &pressedAt},
			wantStatus:    viewmodel.TradeStatusBadge{LabelKey: "trade_status_awaiting_partner_confirmation", Variant: "outline"},
			wantStatusKey: "trade_show_status_note_awaiting_partner_confirmation",
		},
		{
			name:                 "相手だけが押した",
			trade:                model.Trade{ProposerUserID: partner.ID, ReceiverUserID: me, Status: model.TradeStatusMatched, ProposerCompletedAt: &pressedAt},
			wantComplete:         true,
			wantPartnerCompleted: true,
			wantStatus:           viewmodel.TradeStatusBadge{LabelKey: "trade_status_awaiting_your_confirmation", Variant: "warning"},
			wantStatusKey:        "trade_show_status_note_awaiting_your_confirmation",
		},
		{
			name:                 "2人とも押して終わった",
			trade:                model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusCompleted, ProposerCompletedAt: &pressedAt, ReceiverCompletedAt: &pressedAt},
			wantPartnerCompleted: true,
			wantStatus:           viewmodel.TradeStatusBadge{LabelKey: "trade_status_completed", Variant: "success"},
		},
		{
			name:       "押していないうちにやめた",
			trade:      model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusCancelled},
			wantStatus: viewmodel.TradeStatusBadge{LabelKey: "trade_status_cancelled", Variant: "outline"},
		},
	}
	for _, tt := range tests {
		got := viewmodel.NewTradeDetail(ctx, me, &tt.trade, partner, nil, nil, nil, nil, nil, 0, true, loc, now)
		if got.CanComplete != tt.wantComplete || got.PartnerCompleted != tt.wantPartnerCompleted || got.Status != tt.wantStatus || got.StatusNoteKey != tt.wantStatusKey {
			t.Errorf("%s: CanComplete = %v, PartnerCompleted = %v, Status = %+v, StatusNoteKey = %q、%v・%v・%+v・%q を期待",
				tt.name, got.CanComplete, got.PartnerCompleted, got.Status, got.StatusNoteKey, tt.wantComplete, tt.wantPartnerCompleted, tt.wantStatus, tt.wantStatusKey)
		}
	}
}

// TestNewTradeDetail_EndActions は、「交換できなかった」をマッチ成立の間だけ、交換をやめる操作を
// マッチ成立でどちらも「交換できた」を押していない間だけ出すことを検証する。
func TestNewTradeDetail_EndActions(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "yuzu"}
	pressedAt := now.Add(-time.Hour)

	tests := []struct {
		name       string
		trade      model.Trade
		wantFail   bool
		wantCancel bool
	}{
		{name: "返事待ち", trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusPending}},
		{name: "だれも押していないマッチ成立", trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched}, wantFail: true, wantCancel: true},
		{name: "自分だけが押したマッチ成立", trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched, ProposerCompletedAt: &pressedAt}, wantFail: true},
		{name: "相手だけが押したマッチ成立", trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched, ReceiverCompletedAt: &pressedAt}, wantFail: true},
		{name: "交換できなかった", trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusFailed}},
	}
	for _, tt := range tests {
		got := viewmodel.NewTradeDetail(ctx, me, &tt.trade, partner, nil, nil, nil, nil, nil, 0, true, loc, now)
		if got.CanFail != tt.wantFail || got.CanCancel != tt.wantCancel {
			t.Errorf("%s: CanFail = %v, CanCancel = %v、%v・%v を期待", tt.name, got.CanFail, got.CanCancel, tt.wantFail, tt.wantCancel)
		}
	}
}

// TestNewTradeDetail_EndReasons は、「交換できなかった」とやめた出来事に、それぞれの選択肢の理由の文言を添え、
// ほかの操作の選択肢の理由は添えないことを検証する。
func TestNewTradeDetail_EndReasons(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "kasumi"}
	trade := &model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusFailed}
	noShow, decidedElsewhere, declineReason := "no_show", "decided_elsewhere", "place_mismatch"
	events := []*model.TradeEvent{
		{ActorUserID: me, Kind: model.TradeEventKindFailed, Reason: &noShow, CreatedAt: now},
		{ActorUserID: partner.ID, Kind: model.TradeEventKindCancelled, Reason: &decidedElsewhere, CreatedAt: now},
		{ActorUserID: partner.ID, Kind: model.TradeEventKindFailed, Reason: &declineReason, CreatedAt: now},
	}

	got := viewmodel.NewTradeDetail(ctx, me, trade, partner, nil, nil, nil, events, nil, 0, true, loc, now)

	wantKeys := []string{"trade_failure_reason_no_show", "trade_cancellation_reason_decided_elsewhere", ""}
	if len(got.Events) != len(wantKeys) {
		t.Fatalf("Events = %+v、%d件を期待", got.Events, len(wantKeys))
	}
	for i, want := range wantKeys {
		if got.Events[i].ReasonKey != want {
			t.Errorf("Events[%d].ReasonKey = %q、期待値 = %q", i, got.Events[i].ReasonKey, want)
		}
	}
}

// TestNewTradeHistoryRows は、終わった交換の行に、終わった日を開いたユーザーのタイムゾーンで添えることを検証する。
func TestNewTradeHistoryRows(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "haru"}
	// 日本時間では9月19日の朝になる。
	endedAt := time.Date(2026, 9, 18, 23, 0, 0, 0, time.UTC)
	lastYear := time.Date(2025, 12, 30, 0, 0, 0, 0, loc)
	trades := []*model.Trade{
		{ID: model.TradeID(uuid.New()), ProposerUserID: partner.ID, ReceiverUserID: me, Status: model.TradeStatusCompleted, EndedAt: &endedAt},
		{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusWithdrawn, EndedAt: &lastYear},
	}
	items := map[model.TradeID][]*model.Item{trades[0].ID: {{UserID: partner.ID}, {UserID: me}, {UserID: me}}}

	rows := viewmodel.NewTradeHistoryRows(ctx, me, trades, map[model.UserID]*model.User{partner.ID: partner}, items, loc, now)

	want := []viewmodel.TradeRow{
		{
			ID: trades[0].ID.String(), PartnerName: "@haru さん", ReceiveQuantity: 1, GiveQuantity: 2,
			Status: viewmodel.TradeStatusBadge{LabelKey: "trade_status_completed", Variant: "success"}, EndedDate: "9月19日",
		},
		{
			ID: trades[1].ID.String(), PartnerName: "@haru さん",
			Status: viewmodel.TradeStatusBadge{LabelKey: "trade_status_withdrawn", Variant: "outline"}, EndedDate: "2025年12月30日",
		},
	}
	if len(rows) != 2 || rows[0] != want[0] || rows[1] != want[1] {
		t.Errorf("NewTradeHistoryRows() = %+v、%+v を期待", rows, want)
	}
}

// TestNewTradeHistoryCounts は、終わった交換の段階ごとの数を、交換できたを先に決まった順に並べ、1件も無い段階を出さないことを検証する。
func TestNewTradeHistoryCounts(t *testing.T) {
	t.Parallel()

	got := viewmodel.NewTradeHistoryCounts(map[model.TradeStatus]int64{
		model.TradeStatusWithdrawn: 1,
		model.TradeStatusDeclined:  0,
		model.TradeStatusCompleted: 12,
		model.TradeStatusCancelled: 2,
	})
	want := []viewmodel.TradeHistoryCount{
		{LabelKey: "trade_history_count_completed", Count: 12},
		{LabelKey: "trade_history_count_cancelled", Count: 2},
		{LabelKey: "trade_history_count_withdrawn", Count: 1},
	}
	if !slices.Equal(got, want) {
		t.Errorf("NewTradeHistoryCounts() = %+v、%+v を期待", got, want)
	}
	if got := viewmodel.NewTradeHistoryCounts(nil); len(got) != 0 {
		t.Errorf("NewTradeHistoryCounts(nil) = %+v、空を期待", got)
	}
}

// TestNewTradeDetail_Ended は、終わった交換を、これまでの交換の中に置き、いつどう終わったかの文と、過去の形の交換の品の見出しを出すことを検証する。
// 取り下げ・お断り・やめたの文は、終えた出来事を起こした人が開いたユーザーか相手かで分ける。
func TestNewTradeDetail_Ended(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "tsumugi"}
	// 日本時間では9月21日の朝になる。
	endedAt := time.Date(2026, 9, 20, 23, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		status      model.TradeStatus
		events      []*model.TradeEvent
		wantNote    string
		wantReceive string
		wantGive    string
	}{
		{
			name:        "交換できた",
			status:      model.TradeStatusCompleted,
			wantNote:    "trade_show_status_note_completed",
			wantReceive: "trade_show_received_heading",
			wantGive:    "trade_show_gave_heading",
		},
		{
			name:        "交換できなかった",
			status:      model.TradeStatusFailed,
			wantNote:    "trade_show_status_note_failed",
			wantReceive: "trade_show_would_receive_heading",
			wantGive:    "trade_show_would_give_heading",
		},
		{
			name:        "自分が取り下げた",
			status:      model.TradeStatusWithdrawn,
			events:      []*model.TradeEvent{{ActorUserID: me, Kind: model.TradeEventKindProposed}, {ActorUserID: me, Kind: model.TradeEventKindWithdrawn}},
			wantNote:    "trade_show_status_note_withdrawn_self",
			wantReceive: "trade_show_would_receive_heading",
			wantGive:    "trade_show_would_give_heading",
		},
		{
			name:        "相手がお断りした",
			status:      model.TradeStatusDeclined,
			events:      []*model.TradeEvent{{ActorUserID: me, Kind: model.TradeEventKindProposed}, {ActorUserID: partner.ID, Kind: model.TradeEventKindDeclined}},
			wantNote:    "trade_show_status_note_declined_partner",
			wantReceive: "trade_show_would_receive_heading",
			wantGive:    "trade_show_would_give_heading",
		},
		{
			name:        "相手がやめた",
			status:      model.TradeStatusCancelled,
			events:      []*model.TradeEvent{{ActorUserID: partner.ID, Kind: model.TradeEventKindApproved}, {ActorUserID: partner.ID, Kind: model.TradeEventKindCancelled}},
			wantNote:    "trade_show_status_note_cancelled_partner",
			wantReceive: "trade_show_would_receive_heading",
			wantGive:    "trade_show_would_give_heading",
		},
		{
			name:        "やめた出来事が無い",
			status:      model.TradeStatusCancelled,
			wantReceive: "trade_show_would_receive_heading",
			wantGive:    "trade_show_would_give_heading",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: partner.ID, Status: tt.status, EndedAt: &endedAt}
			got := viewmodel.NewTradeDetail(ctx, me, trade, partner, nil, nil, nil, tt.events, nil, 0, true, loc, now)
			if !got.Ended || got.EndedDate != "9月21日" || got.StatusNoteKey != tt.wantNote {
				t.Errorf("Ended = %v, EndedDate = %q, StatusNoteKey = %q、終わった交換・9月21日・%q を期待", got.Ended, got.EndedDate, got.StatusNoteKey, tt.wantNote)
			}
			if got.ReceiveHeadingKey != tt.wantReceive || got.GiveHeadingKey != tt.wantGive {
				t.Errorf("ReceiveHeadingKey = %q, GiveHeadingKey = %q、%q・%q を期待", got.ReceiveHeadingKey, got.GiveHeadingKey, tt.wantReceive, tt.wantGive)
			}
		})
	}

	inProgress := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched}
	got := viewmodel.NewTradeDetail(ctx, me, inProgress, partner, nil, nil, nil, nil, nil, 0, true, loc, now)
	if got.Ended || got.EndedDate != "" || got.ReceiveHeadingKey != "trade_show_receive_heading" || got.GiveHeadingKey != "trade_show_give_heading" {
		t.Errorf("進行中の交換: Ended = %v, EndedDate = %q, ReceiveHeadingKey = %q, GiveHeadingKey = %q、終わっておらず、今の形の見出しを期待",
			got.Ended, got.EndedDate, got.ReceiveHeadingKey, got.GiveHeadingKey)
	}
}

// TestNewTradeDetail_WithdrawnPartner は、相手が退会した交換で、相手の名前とこれまでの流れの主語を退会したユーザーにし、
// プロフィールへのリンクに使うアットネームを空にすることを検証する。
func TestNewTradeDetail_WithdrawnPartner(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	withdrawn := &model.User{ID: model.UserID(uuid.New()), Atname: "deleted-x", DeletedAt: new(now)}
	endedAt := time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC)
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: withdrawn.ID, ReceiverUserID: me, Status: model.TradeStatusWithdrawn, EndedAt: &endedAt}
	events := []*model.TradeEvent{{ActorUserID: withdrawn.ID, Kind: model.TradeEventKindWithdrawn, CreatedAt: endedAt}}

	detail := viewmodel.NewTradeDetail(ctx, me, trade, withdrawn, nil, nil, nil, events, nil, 0, true, loc, now)

	if detail.PartnerName != "退会したユーザー" || detail.PartnerAtname != "" {
		t.Errorf("PartnerName = %q, PartnerAtname = %q、退会したユーザーと空を期待", detail.PartnerName, detail.PartnerAtname)
	}
	if len(detail.Events) != 1 || detail.Events[0].ActorName != "退会したユーザー" || detail.Events[0].TextKey != "trade_event_withdrawn_partner" {
		t.Errorf("Events = %+v、退会したユーザーが取り下げた出来事を期待", detail.Events)
	}
	if detail.StatusNoteKey != "trade_show_status_note_withdrawn_partner" {
		t.Errorf("StatusNoteKey = %q、相手が取り下げた文を期待", detail.StatusNoteKey)
	}
}
