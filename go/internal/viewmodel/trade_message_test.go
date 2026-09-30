package viewmodel_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// TestNewTradeMessageThread は、メッセージと出来事を時刻順に、開いたユーザーのタイムゾーンの日ごとに分けて並べ、
// 同じ時刻では出来事を先に置くことと、自分と相手のどちらのものかを見分けることを検証する。
// 取り消したメッセージの本文は出さない。前に読んだところより後の相手のメッセージを未読とし、最初のものを未読の始まりにする。
func TestNewTradeMessageThread(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("タイムゾーンの読み込みに失敗しました: %v", err)
	}
	viewerID := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "yuzu"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: viewerID, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched}
	items := []*model.Item{{UserID: partner.ID}, {UserID: viewerID}, {UserID: viewerID}}
	// 2026-09-21 20:03 (日本時間) に申し込み、同じ時刻にひとことを送った。相手は日本時間の翌日に承認して返信した。
	proposedAt := time.Date(2026, 9, 21, 11, 3, 0, 0, time.UTC)
	approvedAt := time.Date(2026, 9, 21, 15, 30, 0, 0, time.UTC)
	events := []*model.TradeEvent{
		{ActorUserID: viewerID, Kind: model.TradeEventKindProposed, CreatedAt: proposedAt},
		{ActorUserID: partner.ID, Kind: model.TradeEventKindApproved, CreatedAt: approvedAt},
	}
	firstID := model.TradeMessageID(uuid.New())
	readMessageID := model.TradeMessageID(uuid.New())
	messages := []*model.TradeMessage{
		{ID: firstID, SenderUserID: viewerID, Body: "はじめまして。\n土日の昼なら動けます。", CreatedAt: proposedAt},
		// 前に開いたときに読んでいた、相手のメッセージ。
		{ID: readMessageID, SenderUserID: partner.ID, Body: "承認しました", CreatedAt: approvedAt},
		{SenderUserID: partner.ID, Body: "取り消した本文", RetractedAt: new(approvedAt.Add(3 * time.Minute)), CreatedAt: approvedAt.Add(time.Minute)},
		{SenderUserID: partner.ID, Body: "よろしくお願いします", CreatedAt: approvedAt.Add(2 * time.Minute)},
		{SenderUserID: partner.ID, Body: "東口でお願いします", CreatedAt: approvedAt.Add(4 * time.Minute)},
	}

	thread := viewmodel.NewTradeMessageThread(ctx, viewerID, trade, partner, items, messages, events, &model.TradeMessageReadPosition{CreatedAt: approvedAt, MessageID: readMessageID}, loc, time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC))

	if thread.ID != trade.ID.String() || thread.PartnerName != "@yuzu さん" || !thread.InProgress || thread.ReceiveQuantity != 1 || thread.GiveQuantity != 2 {
		t.Errorf("NewTradeMessageThread() = %+v、相手・進行中・もらう1点・渡す2点を期待", thread)
	}
	if thread.UnreadCount != 2 {
		t.Errorf("UnreadCount = %d、取り消したものを除く未読の2通を期待", thread.UnreadCount)
	}
	if thread.Status.LabelKey != "trade_status_matched" {
		t.Errorf("Status = %+v、マッチ成立を期待", thread.Status)
	}
	if len(thread.Days) != 2 || thread.Days[0].Date != "9月21日" || thread.Days[1].Date != "9月22日" {
		t.Fatalf("Days = %+v、日本時間の9月21日と9月22日を期待", thread.Days)
	}

	first := thread.Days[0].Entries
	if len(first) != 2 {
		t.Fatalf("9月21日の並び = %+v、申し込みとひとことを期待", first)
	}
	if first[0] != (viewmodel.TradeMessageEntry{EventTextKey: "trade_event_proposed_self"}) {
		t.Errorf("1つ目 = %+v、自分の申し込みの出来事を期待", first[0])
	}
	want := viewmodel.TradeMessageEntry{ID: firstID.String(), Mine: true, Body: "はじめまして。\n土日の昼なら動けます。", Time: "20:03", DateTime: "2026-09-21T20:03:00+09:00"}
	if first[1] != want {
		t.Errorf("2つ目 = %+v、%+v を期待", first[1], want)
	}
	if !first[1].CanRetract() || first[0].CanRetract() {
		t.Error("自分のメッセージを取り消せないか、出来事を取り消せる")
	}

	second := thread.Days[1].Entries
	if len(second) != 5 {
		t.Fatalf("9月22日の並び = %+v、承認・読んだ返信・取り消したメッセージ・未読の2通を期待", second)
	}
	if second[0] != (viewmodel.TradeMessageEntry{EventTextKey: "trade_event_approved_partner", ActorName: "@yuzu さん"}) {
		t.Errorf("1つ目 = %+v、相手の承認の出来事を期待", second[0])
	}
	if second[1].Mine || second[1].Body != "承認しました" || second[1].Time != "00:30" || second[1].UnreadStart || second[1].CanRetract() {
		t.Errorf("2つ目 = %+v、読んでいた相手の00:30のメッセージを期待", second[1])
	}
	if !second[2].Retracted || second[2].Body != "" || second[2].UnreadStart {
		t.Errorf("3つ目 = %+v、本文を出さず、未読に数えない取り消したメッセージを期待", second[2])
	}
	if second[3].Body != "よろしくお願いします" || !second[3].UnreadStart || second[4].UnreadStart {
		t.Errorf("4つ目 = %+v, 5つ目 = %+v、4つ目だけを未読の始まりにすることを期待", second[3], second[4])
	}
}

// TestNewTradeMessageThread_Ended は、終わった交換を進行中でないとし、メッセージも出来事も無い交換では日を持たないことを検証する。
func TestNewTradeMessageThread_Ended(t *testing.T) {
	t.Parallel()

	viewerID := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "haru"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: partner.ID, ReceiverUserID: viewerID, Status: model.TradeStatusCompleted}

	thread := viewmodel.NewTradeMessageThread(i18n.SetLocale(context.Background(), i18n.LangJa), viewerID, trade, partner, nil, nil, nil, nil, time.UTC, time.Now())

	if thread.InProgress || len(thread.Days) != 0 {
		t.Errorf("NewTradeMessageThread() = %+v、進行中でなく日を持たないことを期待", thread)
	}
}

// TestNewTradeMessageThread_Guide は、送信の欄の下の案内を、交換の段階と開いたユーザーが動く番かどうかで選び、
// 次にすることが無い段階では出さないことを検証する。
func TestNewTradeMessageThread_Guide(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	me := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "mio"}
	pressedAt := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		trade model.Trade
		want  string
	}{
		{
			name:  "申し込んで相手の返事待ち",
			trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusPending},
			want:  "trade_message_index_guide_awaiting_partner_reply",
		},
		{
			name:  "申し込まれてあなたの返事待ち",
			trade: model.Trade{ProposerUserID: partner.ID, ReceiverUserID: me, Status: model.TradeStatusPending},
			want:  "trade_message_index_guide_awaiting_your_reply",
		},
		{
			name:  "マッチ成立でだれも押していない",
			trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched},
		},
		{
			name:  "相手だけが押したあなたの確認待ち",
			trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusMatched, ReceiverCompletedAt: &pressedAt},
			want:  "trade_message_index_guide_awaiting_your_confirmation",
		},
		{
			name:  "自分だけが押した相手の確認待ち",
			trade: model.Trade{ProposerUserID: partner.ID, ReceiverUserID: me, Status: model.TradeStatusMatched, ReceiverCompletedAt: &pressedAt},
		},
		{
			name:  "終わった交換",
			trade: model.Trade{ProposerUserID: me, ReceiverUserID: partner.ID, Status: model.TradeStatusWithdrawn},
		},
	}
	for _, tt := range tests {
		got := viewmodel.NewTradeMessageThread(ctx, me, &tt.trade, partner, nil, nil, nil, nil, time.UTC, pressedAt)
		if got.GuideKey != tt.want {
			t.Errorf("%s: GuideKey = %q、%q を期待", tt.name, got.GuideKey, tt.want)
		}
	}
}

// TestNewTradeMessageThread_EventReason は、お断りの出来事に選んだ理由の文言の翻訳キーを添え、
// 今の選択肢に無い理由には添えないことを検証する。
func TestNewTradeMessageThread_EventReason(t *testing.T) {
	t.Parallel()

	viewerID := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "tsumugi"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: viewerID, ReceiverUserID: partner.ID, Status: model.TradeStatusDeclined}
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	reason, unknown := string(model.TradeDeclineReasonAlreadyDecided), "removed_choice"
	events := []*model.TradeEvent{
		{ActorUserID: partner.ID, Kind: model.TradeEventKindDeclined, Reason: &reason, CreatedAt: at},
		{ActorUserID: partner.ID, Kind: model.TradeEventKindDeclined, Reason: &unknown, CreatedAt: at.Add(time.Minute)},
	}

	thread := viewmodel.NewTradeMessageThread(i18n.SetLocale(context.Background(), i18n.LangJa), viewerID, trade, partner, nil, nil, events, nil, time.UTC, at)

	entries := thread.Days[0].Entries
	if entries[0].ReasonKey != "trade_decline_reason_already_decided" || entries[1].ReasonKey != "" {
		t.Errorf("Entries = %+v、1つ目にだけ「もう交換が決まった」の翻訳キーを期待", entries)
	}
}

// TestNewTradeMessageThread_NeverRead は、まだ読んだことが無いときは、相手の最初のメッセージを未読の始まりにすることを検証する。
func TestNewTradeMessageThread_NeverRead(t *testing.T) {
	t.Parallel()

	viewerID := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "haru"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: partner.ID, ReceiverUserID: viewerID, Status: model.TradeStatusPending}
	sentAt := time.Date(2026, 9, 21, 11, 3, 0, 0, time.UTC)
	messages := []*model.TradeMessage{
		{SenderUserID: partner.ID, Body: "はじめまして", CreatedAt: sentAt},
		{SenderUserID: viewerID, Body: "よろしくお願いします", CreatedAt: sentAt.Add(time.Minute)},
	}

	thread := viewmodel.NewTradeMessageThread(i18n.SetLocale(context.Background(), i18n.LangJa), viewerID, trade, partner, nil, messages, nil, nil, time.UTC, sentAt)

	entries := thread.Days[0].Entries
	if thread.UnreadCount != 1 || !entries[0].UnreadStart || entries[1].UnreadStart {
		t.Errorf("UnreadCount = %d, Entries = %+v、相手の1通目だけを未読の始まりにすることを期待", thread.UnreadCount, entries)
	}
}

// TestNewTradeMessageThread_SameTimestamp は、同じ時刻でも読んだIDより後に並ぶメッセージだけを未読にすることを検証する。
func TestNewTradeMessageThread_SameTimestamp(t *testing.T) {
	t.Parallel()

	viewerID := model.UserID(uuid.New())
	partner := &model.User{ID: model.UserID(uuid.New()), Atname: "haru"}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: partner.ID, ReceiverUserID: viewerID, Status: model.TradeStatusPending}
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	firstID, secondID := model.TradeMessageID(uuid.New()), model.TradeMessageID(uuid.New())
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}
	messages := []*model.TradeMessage{
		{ID: firstID, SenderUserID: partner.ID, CreatedAt: at},
		{ID: secondID, SenderUserID: partner.ID, CreatedAt: at},
	}
	position := &model.TradeMessageReadPosition{CreatedAt: at, MessageID: firstID}
	thread := viewmodel.NewTradeMessageThread(i18n.SetLocale(context.Background(), i18n.LangJa), viewerID, trade, partner, nil, messages, nil, position, time.UTC, at)
	if thread.UnreadCount != 1 || thread.Days[0].Entries[0].UnreadStart || !thread.Days[0].Entries[1].UnreadStart {
		t.Errorf("同時刻の未読 = %+v、2通目だけ未読を期待", thread)
	}
}

// TestNewTradeMessageListRows は、交換を並び順のままメッセージの一覧の行にし、最新のメッセージを抜き出して、
// 今日のものは時刻・それより前のものは日付で示すことと、終わった交換を見分けることを検証する。
func TestNewTradeMessageListRows(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	yuzu := &model.User{ID: model.UserID(uuid.New()), Atname: "yuzu"}
	haru := &model.User{ID: model.UserID(uuid.New()), Atname: "haru"}
	inProgress := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: yuzu.ID, Status: model.TradeStatusMatched}
	ended := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: haru.ID, ReceiverUserID: me, Status: model.TradeStatusCompleted}
	withoutMessage := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: haru.ID, Status: model.TradeStatusPending, CreatedAt: time.Date(2026, 9, 29, 2, 15, 0, 0, time.UTC)}
	partners := map[model.UserID]*model.User{yuzu.ID: yuzu, haru.ID: haru}
	items := map[model.TradeID][]*model.Item{inProgress.ID: {{UserID: yuzu.ID}, {UserID: me}, {UserID: me}}}
	latestMessages := map[model.TradeID]*model.TradeMessage{
		// 日本時間では今日の10:05になる。
		inProgress.ID: {SenderUserID: yuzu.ID, Body: "東口でお願いします", CreatedAt: time.Date(2026, 9, 29, 1, 5, 0, 0, time.UTC)},
		ended.ID:      {SenderUserID: me, Body: "取り消した本文", RetractedAt: new(now), CreatedAt: time.Date(2026, 9, 18, 6, 40, 0, 0, time.UTC)},
	}
	unreadCounts := map[model.TradeID]int64{inProgress.ID: 2}

	rows := viewmodel.NewTradeMessageListRows(ctx, me, []*model.Trade{inProgress, ended, withoutMessage}, partners, items, latestMessages, unreadCounts, loc, now)

	if len(rows) != 3 {
		t.Fatalf("行 = %+v、3行を期待", rows)
	}
	want := viewmodel.TradeMessageListRow{
		ID:              inProgress.ID.String(),
		PartnerName:     "@yuzu さん",
		ReceiveQuantity: 1,
		GiveQuantity:    2,
		LatestMessage:   &viewmodel.TradeMessagePreview{Body: "東口でお願いします", SentAt: "10:05", DateTime: "2026-09-29T10:05:00+09:00"},
		UnreadCount:     2,
	}
	if got := rows[0]; got.ID != want.ID || got.PartnerName != want.PartnerName || got.ReceiveQuantity != 1 || got.GiveQuantity != 2 ||
		got.Ended || got.UnreadCount != 2 || got.LatestMessage == nil || *got.LatestMessage != *want.LatestMessage {
		t.Errorf("1行目 = %+v (最新 %+v)、%+v を期待", got, got.LatestMessage, want)
	}
	if got := rows[1]; got.PartnerName != "@haru さん" || !got.Ended || got.UnreadCount != 0 || got.LatestMessage == nil ||
		*got.LatestMessage != (viewmodel.TradeMessagePreview{Mine: true, Retracted: true, SentAt: "9月18日", DateTime: "2026-09-18T15:40:00+09:00"}) {
		t.Errorf("2行目 = %+v (最新 %+v)、本文を出さない自分の取り消したメッセージを日付で示す、終わった交換を期待", got, got.LatestMessage)
	}
	if got := rows[2]; got.LatestMessage != nil || got.Ended || got.DisplayTime != "11:15" || got.DateTime != "2026-09-29T11:15:00+09:00" {
		t.Errorf("3行目 = %+v、申し込んだ日時を示すメッセージの無い進行中の交換を期待", got)
	}
}

// TestTradeMessageWithdrawnPartner は、メッセージのページと一覧で、退会した相手を退会したユーザーとして示し、
// 出来事のお知らせの主語も同じ名前にすることを検証する。
func TestTradeMessageWithdrawnPartner(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	loc := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, loc)
	me := model.UserID(uuid.New())
	withdrawn := &model.User{ID: model.UserID(uuid.New()), Atname: "deleted-x", DeletedAt: new(now)}
	trade := &model.Trade{ID: model.TradeID(uuid.New()), ProposerUserID: me, ReceiverUserID: withdrawn.ID, Status: model.TradeStatusDeclined}
	events := []*model.TradeEvent{{ActorUserID: withdrawn.ID, Kind: model.TradeEventKindDeclined, CreatedAt: now}}

	thread := viewmodel.NewTradeMessageThread(ctx, me, trade, withdrawn, nil, nil, events, nil, loc, now)
	if thread.PartnerName != "退会したユーザー" {
		t.Errorf("PartnerName = %q、退会したユーザーを期待", thread.PartnerName)
	}
	if len(thread.Days) != 1 || len(thread.Days[0].Entries) != 1 || thread.Days[0].Entries[0].ActorName != "退会したユーザー" {
		t.Errorf("Days = %+v、退会したユーザーがお断りしたお知らせを期待", thread.Days)
	}

	rows := viewmodel.NewTradeMessageListRows(ctx, me, []*model.Trade{trade}, map[model.UserID]*model.User{withdrawn.ID: withdrawn}, nil, nil, nil, loc, now)
	if len(rows) != 1 || rows[0].PartnerName != "退会したユーザー" {
		t.Errorf("NewTradeMessageListRows() = %+v、相手の名前が退会したユーザーの行を期待", rows)
	}
}
