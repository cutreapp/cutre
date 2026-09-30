package viewmodel

import (
	"context"
	"slices"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeMessageEntry は、交換のメッセージのページの流れに並ぶ、メッセージか出来事のお知らせの1つ。
type TradeMessageEntry struct {
	// ID はメッセージのIDで、取り消しの送信先に使う。出来事のお知らせでは空。
	ID string
	// EventTextKey は出来事のお知らせの文の翻訳キーで、メッセージでは空。文は出来事を起こした人 (ActorName) を主語にする。
	EventTextKey string
	// ActorName は出来事を起こした人の名前 (@アットネームか、退会したユーザー)。開いたユーザー自身のときは空。
	ActorName string
	// ReasonKey は出来事で選んだ理由の文言の翻訳キー。理由を選ばない出来事とメッセージでは空。
	ReasonKey string
	// Mine は、開いたユーザーが送ったメッセージか。
	Mine bool
	// Body はメッセージの本文。取り消したメッセージでは空にし、画面にもHTMLにも出さない。
	Body string
	// Retracted は、送った人がメッセージを取り消したか。
	Retracted bool
	// Time はメッセージを送った時刻を、開いたユーザーのタイムゾーンで表示用に整えたもの。
	Time string
	// DateTime はメッセージを送った日時の機械向けの表記 (RFC 3339)。<time> の datetime に使う。
	DateTime string
	// UnreadStart は、開いたユーザーがまだ読んでいなかった最初のメッセージか。この前に未読の境目を置く。
	UnreadStart bool
}

// IsEvent は出来事のお知らせかを返す。
func (e TradeMessageEntry) IsEvent() bool {
	return e.EventTextKey != ""
}

// CanRetract は、開いたユーザーがメッセージを取り消せるかを返す。自分が送った、まだ取り消していないメッセージに限る。
func (e TradeMessageEntry) CanRetract() bool {
	return !e.IsEvent() && e.Mine && !e.Retracted
}

// TradeMessageDay は、交換のメッセージのページの流れを、開いたユーザーのタイムゾーンの日ごとに分けたもの。
type TradeMessageDay struct {
	// Date はその日を表示用に整えたもの。
	Date    string
	Entries []TradeMessageEntry
}

// TradeMessageThread は交換のメッセージのページに出す、開いたユーザーから見た交換とメッセージの流れ。
type TradeMessageThread struct {
	// ID は交換のID。送信先と交換のページへのリンクに使う。
	ID string
	// PartnerName は交換の相手の名前 (@アットネームか、退会したユーザー)。
	PartnerName string
	Status      TradeStatusBadge
	// ReceiveQuantity・GiveQuantity は、もらう品・渡す品の点数。
	ReceiveQuantity int
	GiveQuantity    int
	// InProgress は交換が進行中か。進行中の交換でだけメッセージを送れる。
	InProgress bool
	// Days はメッセージと出来事を、時刻順に日ごとに分けたもの。
	Days []TradeMessageDay
	// UnreadCount は、開いたユーザーがまだ読んでいなかったメッセージの数。
	UnreadCount int
	// GuideKey は、送信の欄の下に出す、交換の段階と開いたユーザーに合わせた案内の翻訳キー。案内の無い段階では空。
	GuideKey string
}

// NewTradeMessageThread は、ユーザー viewerID が開いた交換 trade のメッセージと出来事を、時刻順に日ごとに分けて並べる。
//
// partner は交換の相手、items は交換の品のアイテムで、点数に使う。
// 同じ時刻のメッセージと出来事は、出来事を先に置く。申し込みの出来事と、ひとことの1通目は同じトランザクションで同じ時刻に記録するため。
// lastReadPosition は開いたユーザーが前に読んだところ (読んだことが無ければnil) で、それより後に相手が送った最初のメッセージを未読の始まりにする。
// 取り消したメッセージは未読に数えないため、未読の始まりにもしない。
// 日と時刻は loc のタイムゾーンで示し、日は now と同じ年なら年を省く。
func NewTradeMessageThread(
	ctx context.Context,
	viewerID model.UserID,
	trade *model.Trade,
	partner *model.User,
	items []*model.Item,
	messages []*model.TradeMessage,
	events []*model.TradeEvent,
	lastReadPosition *model.TradeMessageReadPosition,
	loc *time.Location,
	now time.Time,
) TradeMessageThread {
	thread := TradeMessageThread{
		ID:          trade.ID.String(),
		PartnerName: tradePartnerName(ctx, partner),
		Status:      newTradeStatusBadge(trade, viewerID),
		InProgress:  trade.IsInProgress(),
		GuideKey:    tradeMessageGuideKey(trade, viewerID),
	}
	for _, item := range items {
		if item.UserID == viewerID {
			thread.GiveQuantity++
		} else {
			thread.ReceiveQuantity++
		}
	}

	type timedEntry struct {
		at    time.Time
		entry TradeMessageEntry
	}
	entries := make([]timedEntry, 0, len(events)+len(messages))
	for _, event := range events {
		keys := tradeEventTextKeys[event.Kind]
		entry := TradeMessageEntry{EventTextKey: keys.self, ReasonKey: tradeEventReasonKey(event)}
		if event.ActorUserID != viewerID {
			entry.EventTextKey = keys.partner
			entry.ActorName = thread.PartnerName
		}
		entries = append(entries, timedEntry{at: event.CreatedAt, entry: entry})
	}
	timeLayout := i18n.T(ctx, "time_hour_minute_layout")
	for _, message := range messages {
		createdAt := message.CreatedAt.In(loc)
		entry := TradeMessageEntry{
			ID:        message.ID.String(),
			Mine:      message.SenderUserID == viewerID,
			Retracted: message.RetractedAt != nil,
			Time:      createdAt.Format(timeLayout),
			DateTime:  createdAt.Format(time.RFC3339),
		}
		if !entry.Retracted {
			entry.Body = message.Body
		}
		// メッセージは送った順に並んでいるため、最初に見つけた未読のものが境目になる。
		if isUnreadMessage(message, viewerID, lastReadPosition) {
			entry.UnreadStart = thread.UnreadCount == 0
			thread.UnreadCount++
		}
		entries = append(entries, timedEntry{at: message.CreatedAt, entry: entry})
	}
	// 出来事を先に入れてあるため、安定ソートで同じ時刻の出来事がメッセージの前に残る。
	slices.SortStableFunc(entries, func(a, b timedEntry) int { return a.at.Compare(b.at) })

	var lastDay string
	for _, e := range entries {
		at := e.at.In(loc)
		if day := at.Format(time.DateOnly); day != lastDay {
			thread.Days = append(thread.Days, TradeMessageDay{Date: FormatDate(ctx, at, now.In(loc))})
			lastDay = day
		}
		last := &thread.Days[len(thread.Days)-1]
		last.Entries = append(last.Entries, e.entry)
	}

	return thread
}

// tradeMessageGuideKey は、ユーザー viewerID が開いた交換 trade のメッセージのページで、送信の欄の下に出す案内の翻訳キーを返す。
//
// 相手の返事待ちでは承認の前でも送れることを、あなたの返事待ちとあなたの確認待ちでは交換のページでする操作を伝える。
// マッチ成立のあとで2人とも「交換できた」を押していないときと、相手の確認待ちのときは、次にすることを案内しないため空を返す。
func tradeMessageGuideKey(trade *model.Trade, viewerID model.UserID) string {
	switch trade.Status {
	case model.TradeStatusPending:
		if trade.IsAwaiting(viewerID) {
			return "trade_message_index_guide_awaiting_your_reply"
		}
		return "trade_message_index_guide_awaiting_partner_reply"
	case model.TradeStatusMatched:
		if trade.IsAwaiting(viewerID) {
			return "trade_message_index_guide_awaiting_your_confirmation"
		}
	}

	return ""
}

// isUnreadMessage は、メッセージ message が、位置 lastReadPosition まで読んだユーザー viewerID にとって未読かを返す。
// 相手が送った、取り消していないメッセージのうち、最後に読んだ位置より後に並ぶものが当たる。読んだことが無ければすべてが当たる。
// 数えるクエリ (CountUnreadTradeMessagesByTradeIDs) と条件を揃える。
func isUnreadMessage(message *model.TradeMessage, viewerID model.UserID, lastReadPosition *model.TradeMessageReadPosition) bool {
	if message.SenderUserID == viewerID || message.RetractedAt != nil {
		return false
	}

	return lastReadPosition == nil || message.CreatedAt.After(lastReadPosition.CreatedAt) ||
		(message.CreatedAt.Equal(lastReadPosition.CreatedAt) && message.ID.String() > lastReadPosition.MessageID.String())
}

// TradeMessagePreview は、メッセージの一覧と交換のページに出す、交換の最新のメッセージの抜き出し。
type TradeMessagePreview struct {
	// Mine は、開いたユーザーが送ったメッセージか。
	Mine bool
	// Body はメッセージの本文。取り消したメッセージでは空にし、画面にもHTMLにも出さない。
	Body string
	// Retracted は、送った人がメッセージを取り消したか。
	Retracted bool
	// SentAt はメッセージを送った日時を、開いたユーザーのタイムゾーンで表示用に整えたもの。今日なら時刻、それより前なら日付にする。
	SentAt string
	// DateTime はメッセージを送った日時の機械向けの表記 (RFC 3339)。<time> の datetime に使う。
	DateTime string
}

// newTradeMessagePreview は、ユーザー viewerID が見る交換の最新のメッセージ message を抜き出す。message がnilならnilを返す。
// 日時は loc のタイムゾーンで示し、now と同じ日なら時刻を、違う日なら日付を返す。
func newTradeMessagePreview(ctx context.Context, viewerID model.UserID, message *model.TradeMessage, loc *time.Location, now time.Time) *TradeMessagePreview {
	if message == nil {
		return nil
	}

	createdAt := message.CreatedAt.In(loc)
	preview := &TradeMessagePreview{
		Mine:      message.SenderUserID == viewerID,
		Retracted: message.RetractedAt != nil,
		DateTime:  createdAt.Format(time.RFC3339),
	}
	if !preview.Retracted {
		preview.Body = message.Body
	}
	preview.SentAt = formatListTime(ctx, message.CreatedAt, loc, now)

	return preview
}

// formatListTime は、一覧に出す日時 at を loc のタイムゾーンで、now と同じ日なら時刻、違う日なら日付に整える。
func formatListTime(ctx context.Context, at time.Time, loc *time.Location, now time.Time) string {
	at, now = at.In(loc), now.In(loc)
	if at.Format(time.DateOnly) == now.Format(time.DateOnly) {
		return at.Format(i18n.T(ctx, "time_hour_minute_layout"))
	}

	return FormatDate(ctx, at, now)
}

// TradeMessageListRow は、メッセージの一覧の1行。交換ごとに1行にする。
type TradeMessageListRow struct {
	// ID は交換のID。交換のメッセージのページへのリンクに使う。
	ID string
	// PartnerName は交換の相手の名前 (@アットネームか、退会したユーザー)。
	PartnerName string
	// ReceiveQuantity・GiveQuantity は、もらう品・渡す品の点数。
	ReceiveQuantity int
	GiveQuantity    int
	// Ended は交換が終わったか。終わった交換には、そのことを示すバッジを添える。
	Ended bool
	// LatestMessage は交換の最新のメッセージ。メッセージが無ければnil。
	LatestMessage *TradeMessagePreview
	// DisplayTime と DateTime は一覧の行の日時。メッセージが無ければ申し込まれた日時を使う。
	DisplayTime string
	DateTime    string
	// UnreadCount は開いたユーザーの未読のメッセージの数。
	UnreadCount int64
}

// NewTradeMessageListRows は、ユーザー viewerID の交換を、交換の並び順のままメッセージの一覧の行にする。
//
// partners には交換の相手を、items には交換ごとの品のアイテムを、latestMessages には交換ごとの最新のメッセージを、
// unreadCounts には交換ごとの未読のメッセージの数を入れて渡す。
// 日時は loc のタイムゾーンで示す。
func NewTradeMessageListRows(
	ctx context.Context,
	viewerID model.UserID,
	trades []*model.Trade,
	partners map[model.UserID]*model.User,
	items map[model.TradeID][]*model.Item,
	latestMessages map[model.TradeID]*model.TradeMessage,
	unreadCounts map[model.TradeID]int64,
	loc *time.Location,
	now time.Time,
) []TradeMessageListRow {
	rows := make([]TradeMessageListRow, len(trades))
	for i, trade := range trades {
		row := TradeMessageListRow{
			ID:            trade.ID.String(),
			PartnerName:   tradePartnerName(ctx, partners[trade.PartnerUserID(viewerID)]),
			Ended:         !trade.IsInProgress(),
			LatestMessage: newTradeMessagePreview(ctx, viewerID, latestMessages[trade.ID], loc, now),
			UnreadCount:   unreadCounts[trade.ID],
		}
		if row.LatestMessage != nil {
			row.DisplayTime = row.LatestMessage.SentAt
			row.DateTime = row.LatestMessage.DateTime
		} else {
			row.DisplayTime = formatListTime(ctx, trade.CreatedAt, loc, now)
			row.DateTime = trade.CreatedAt.In(loc).Format(time.RFC3339)
		}
		for _, item := range items[trade.ID] {
			if item.UserID == viewerID {
				row.GiveQuantity++
			} else {
				row.ReceiveQuantity++
			}
		}
		rows[i] = row
	}

	return rows
}
