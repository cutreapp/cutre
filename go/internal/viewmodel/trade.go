package viewmodel

import (
	"context"
	"slices"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeItemRow は、交換の品 (にできるもの) として出すアイテムの1行。
type TradeItemRow struct {
	EventCategoryName string
	GoodsName         string
	// Note はアイテムのひとこと。入れていなければ空。
	Note string
}

// newTradeItemRow はアイテムを、カテゴリーとグッズの名前とひとことの行にする。無いマスタの名前は空にする。
func newTradeItemRow(item *model.Item, goods map[model.GoodsID]*model.Goods, categories map[model.EventCategoryID]*model.EventCategory) TradeItemRow {
	row := TradeItemRow{Note: item.Note}
	if g := goods[item.GoodsID]; g != nil {
		row.GoodsName = g.Name
		if category := categories[g.EventCategoryID]; category != nil {
			row.EventCategoryName = category.Name
		}
	}

	return row
}

// tradePartnerName は交換の相手 partner を、画面に出す名前にする。
//
// 退会した相手は「退会したユーザー」にする。退会した人のアットネームは匿名化した値のため出さない。
// 相手の行は退会しても残るため見つからないことは無いが、見つからないときも同じく退会したユーザーとして示す。
func tradePartnerName(ctx context.Context, partner *model.User) string {
	if partner == nil || partner.DeletedAt != nil {
		return i18n.T(ctx, "trade_partner_name_withdrawn")
	}

	return i18n.T(ctx, "trade_partner_name", map[string]any{"Atname": partner.Atname})
}

// TradeItemChoice は、交換を申し込む画面に出す、交換の品にできるアイテムの1つ。
type TradeItemChoice struct {
	// ID はアイテムのID。選んだアイテムとしてフォームで送る。
	ID string
	TradeItemRow
	// Checked は組み合わせに選んでいるか。
	Checked bool
}

// TradeProposal は、交換を申し込む画面 (組み合わせを選ぶ・申し込み内容の確認) に出す、申し込む相手と交換の品にできるアイテム。
type TradeProposal struct {
	// Atname は申し込む相手のアットネーム。
	Atname string
	// Receivable はもらうもの (相手の譲れるアイテム)、Givable は渡すもの (自分の譲れるアイテム)。
	Receivable []TradeItemChoice
	Givable    []TradeItemChoice
}

// ReceiveItemIDs は、もらうもののうち選んでいるアイテムのIDを返す。
func (p TradeProposal) ReceiveItemIDs() []string {
	return checkedTradeItemIDs(p.Receivable)
}

// GiveItemIDs は、渡すもののうち選んでいるアイテムのIDを返す。
func (p TradeProposal) GiveItemIDs() []string {
	return checkedTradeItemIDs(p.Givable)
}

// NewTradeProposal は、アットネーム atname の相手との間で交換できるアイテム match を、交換を申し込む画面の選択肢にする。
// receiveIDs・giveIDs に入っているアイテムを選んでいる状態にする。goods・categories には match のアイテムのマスタをすべて入れて渡す。
func NewTradeProposal(
	atname string,
	match model.Match,
	goods map[model.GoodsID]*model.Goods,
	categories map[model.EventCategoryID]*model.EventCategory,
	receiveIDs, giveIDs []model.ItemID,
) TradeProposal {
	return TradeProposal{
		Atname:     atname,
		Receivable: newTradeItemChoices(match.Receivable, goods, categories, receiveIDs),
		Givable:    newTradeItemChoices(match.Givable, goods, categories, giveIDs),
	}
}

// newTradeItemChoices は交換できるアイテムを選択肢にし、checkedIDs に入っているものを選んでいる状態にする。
func newTradeItemChoices(items []model.MatchItem, goods map[model.GoodsID]*model.Goods, categories map[model.EventCategoryID]*model.EventCategory, checkedIDs []model.ItemID) []TradeItemChoice {
	checked := make(map[model.ItemID]bool, len(checkedIDs))
	for _, id := range checkedIDs {
		checked[id] = true
	}

	choices := make([]TradeItemChoice, len(items))
	for i, item := range items {
		choices[i] = TradeItemChoice{
			ID:           item.Item.ID.String(),
			TradeItemRow: newTradeItemRow(item.Item, goods, categories),
			Checked:      checked[item.Item.ID],
		}
	}

	return choices
}

// checkedTradeItemIDs は選んでいるアイテムのIDを、選択肢の順に返す。
func checkedTradeItemIDs(choices []TradeItemChoice) []string {
	var ids []string
	for _, choice := range choices {
		if choice.Checked {
			ids = append(ids, choice.ID)
		}
	}

	return ids
}

// TradeStatusBadge は、交換の段階を、開いたユーザーから見た言葉で示すバッジ。
type TradeStatusBadge struct {
	// LabelKey はバッジの文言の翻訳キー。
	LabelKey string
	// Variant はバッジの data-variant の値 (warning・brand・success・outline)。
	Variant string
}

// newTradeStatusBadge は、交換 trade の段階を、ユーザー viewerID から見たバッジにする。
//
// ユーザーの返事や確認を待っている交換は、ユーザーが動く番であることが分かるよう warning にする。
func newTradeStatusBadge(trade *model.Trade, viewerID model.UserID) TradeStatusBadge {
	switch trade.Status {
	case model.TradeStatusPending:
		if trade.IsAwaiting(viewerID) {
			return TradeStatusBadge{LabelKey: "trade_status_awaiting_your_reply", Variant: "warning"}
		}
		return TradeStatusBadge{LabelKey: "trade_status_awaiting_partner_reply", Variant: "outline"}
	case model.TradeStatusMatched:
		if trade.IsAwaiting(viewerID) {
			return TradeStatusBadge{LabelKey: "trade_status_awaiting_your_confirmation", Variant: "warning"}
		}
		if trade.HasCompleted(viewerID) {
			return TradeStatusBadge{LabelKey: "trade_status_awaiting_partner_confirmation", Variant: "outline"}
		}
		return TradeStatusBadge{LabelKey: "trade_status_matched", Variant: "brand"}
	case model.TradeStatusCompleted:
		return TradeStatusBadge{LabelKey: "trade_status_completed", Variant: "success"}
	case model.TradeStatusWithdrawn:
		return TradeStatusBadge{LabelKey: "trade_status_withdrawn", Variant: "outline"}
	case model.TradeStatusDeclined:
		return TradeStatusBadge{LabelKey: "trade_status_declined", Variant: "outline"}
	case model.TradeStatusFailed:
		return TradeStatusBadge{LabelKey: "trade_status_failed", Variant: "outline"}
	default:
		return TradeStatusBadge{LabelKey: "trade_status_cancelled", Variant: "outline"}
	}
}

// TradeRow は交換の一覧の1行。
type TradeRow struct {
	// ID は交換のID。交換のページへのリンクに使う。
	ID string
	// PartnerName は交換の相手の名前 (@アットネームか、退会したユーザー)。
	PartnerName string
	// ReceiveQuantity・GiveQuantity は、もらう品・渡す品の点数。
	ReceiveQuantity int
	GiveQuantity    int
	Status          TradeStatusBadge
	// EndedDate は、終わった交換の終わった日を、開いたユーザーのタイムゾーンで表示用に整えたもの。進行中の交換の一覧では空。
	EndedDate string
}

// NewTradeRows は、ユーザー viewerID の交換を、交換の並び順のまま一覧の行にする。
// partners には交換の相手を、items には交換ごとの品のアイテムを入れて渡す。
func NewTradeRows(ctx context.Context, viewerID model.UserID, trades []*model.Trade, partners map[model.UserID]*model.User, items map[model.TradeID][]*model.Item) []TradeRow {
	rows := make([]TradeRow, len(trades))
	for i, trade := range trades {
		row := TradeRow{
			ID:          trade.ID.String(),
			PartnerName: tradePartnerName(ctx, partners[trade.PartnerUserID(viewerID)]),
			Status:      newTradeStatusBadge(trade, viewerID),
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

// NewTradeHistoryRows は、ユーザー viewerID の終わった交換を、交換の並び順のまま、これまでの交換の一覧の行にする。
// 行には NewTradeRows の内容に加えて、終わった日を loc のタイムゾーンで、now と同じ年なら年を省いて添える。
func NewTradeHistoryRows(
	ctx context.Context,
	viewerID model.UserID,
	trades []*model.Trade,
	partners map[model.UserID]*model.User,
	items map[model.TradeID][]*model.Item,
	loc *time.Location,
	now time.Time,
) []TradeRow {
	rows := NewTradeRows(ctx, viewerID, trades, partners, items)
	for i, trade := range trades {
		if trade.EndedAt != nil {
			rows[i].EndedDate = FormatDate(ctx, trade.EndedAt.In(loc), now.In(loc))
		}
	}

	return rows
}

// TradeHistoryCount は、これまでの交換への行に添える、終わった交換の段階ごとの数の1つ。
type TradeHistoryCount struct {
	// LabelKey は段階と数の文言の翻訳キー。文言は数 (Count) を受け取る。
	LabelKey string
	Count    int64
}

// tradeHistoryCountLabelKeys は、これまでの交換への行に数を出す段階と、その文言の翻訳キー。この順に並べる。
var tradeHistoryCountLabelKeys = []struct {
	status model.TradeStatus
	key    string
}{
	{status: model.TradeStatusCompleted, key: "trade_history_count_completed"},
	{status: model.TradeStatusFailed, key: "trade_history_count_failed"},
	{status: model.TradeStatusCancelled, key: "trade_history_count_cancelled"},
	{status: model.TradeStatusDeclined, key: "trade_history_count_declined"},
	{status: model.TradeStatusWithdrawn, key: "trade_history_count_withdrawn"},
}

// NewTradeHistoryCounts は、終わった交換の段階ごとの数 counts を、これまでの交換への行に添える形にする。
// 交換できたを先に、決まった順に並べ、1件も無い段階は出さない。
func NewTradeHistoryCounts(counts map[model.TradeStatus]int64) []TradeHistoryCount {
	var rows []TradeHistoryCount
	for _, label := range tradeHistoryCountLabelKeys {
		if count := counts[label.status]; count > 0 {
			rows = append(rows, TradeHistoryCount{LabelKey: label.key, Count: count})
		}
	}

	return rows
}

// TradeEventRow は交換のページの「これまでの流れ」の1行。
type TradeEventRow struct {
	// Date は出来事が起きた日を、開いたユーザーのタイムゾーンで表示用に整えたもの。
	Date string
	// TextKey は出来事の文の翻訳キー。文は出来事を起こした人 (ActorName) を主語にする。
	TextKey string
	// ActorName は出来事を起こした人の名前 (@アットネームか、退会したユーザー)。開いたユーザー自身のときは空。
	ActorName string
	// ReasonKey は出来事で選んだ理由の文言の翻訳キー。理由を選ばない出来事では空。
	ReasonKey string
}

// TradeDeclineReasonLabelKey はお断りの理由 reason の文言の翻訳キーを返す。
func TradeDeclineReasonLabelKey(reason model.TradeDeclineReason) string {
	return "trade_decline_reason_" + string(reason)
}

// TradeFailureReasonLabelKey は「交換できなかった」の理由 reason の文言の翻訳キーを返す。
func TradeFailureReasonLabelKey(reason model.TradeFailureReason) string {
	return "trade_failure_reason_" + string(reason)
}

// TradeCancellationReasonLabelKey は交換をやめる理由 reason の文言の翻訳キーを返す。
func TradeCancellationReasonLabelKey(reason model.TradeCancellationReason) string {
	return "trade_cancellation_reason_" + string(reason)
}

// tradeEventReasonKey は出来事 event で選んだ理由の文言の翻訳キーを返す。
// 理由を選ばない出来事と、今の選択肢に無い理由 (選択肢から外したもの) では空を返す。
func tradeEventReasonKey(event *model.TradeEvent) string {
	if event.Reason == nil {
		return ""
	}
	switch event.Kind {
	case model.TradeEventKindDeclined:
		if reason, ok := model.ParseTradeDeclineReason(*event.Reason); ok {
			return TradeDeclineReasonLabelKey(reason)
		}
	case model.TradeEventKindFailed:
		if reason, ok := model.ParseTradeFailureReason(*event.Reason); ok {
			return TradeFailureReasonLabelKey(reason)
		}
	case model.TradeEventKindCancelled:
		if reason, ok := model.ParseTradeCancellationReason(*event.Reason); ok {
			return TradeCancellationReasonLabelKey(reason)
		}
	}

	return ""
}

// tradeEventTextKeys は出来事の種類ごとの文の翻訳キー。開いたユーザー自身が起こしたもの (self) と、相手が起こしたもの (partner) で分ける。
var tradeEventTextKeys = map[model.TradeEventKind]struct{ self, partner string }{
	model.TradeEventKindProposed:  {self: "trade_event_proposed_self", partner: "trade_event_proposed_partner"},
	model.TradeEventKindWithdrawn: {self: "trade_event_withdrawn_self", partner: "trade_event_withdrawn_partner"},
	model.TradeEventKindApproved:  {self: "trade_event_approved_self", partner: "trade_event_approved_partner"},
	model.TradeEventKindDeclined:  {self: "trade_event_declined_self", partner: "trade_event_declined_partner"},
	model.TradeEventKindCompleted: {self: "trade_event_completed_self", partner: "trade_event_completed_partner"},
	model.TradeEventKindFailed:    {self: "trade_event_failed_self", partner: "trade_event_failed_partner"},
	model.TradeEventKindCancelled: {self: "trade_event_cancelled_self", partner: "trade_event_cancelled_partner"},
}

// TradeDetail は交換のページに出す、開いたユーザーから見た交換。
type TradeDetail struct {
	// ID は交換のID。取り下げなどの操作の送信先に使う。
	ID string
	// PartnerName は交換の相手の名前 (@アットネームか、退会したユーザー)。
	PartnerName string
	// PartnerAtname は相手のプロフィールへのリンクに使う、交換の相手のアットネーム。
	// 退会した相手ではプロフィールが無いため空にし、リンクを出さない。
	PartnerAtname string
	Status        TradeStatusBadge
	// Receive はもらう品 (相手のアイテム)、Give は渡す品 (開いたユーザーのアイテム) で、1点ずつ並ぶ。
	Receive []TradeItemRow
	Give    []TradeItemRow
	// Events は、これまでの流れで、起きた順に並ぶ。
	Events []TradeEventRow
	// StatusNoteKey は段階のバッジの横に添える、次にすることの案内か、終わった交換ではいつどう終わったかの文の翻訳キー。案内の無い段階では空。
	// 文言は相手の名前 (Name) と、終わった日 (Date) を受け取れる。
	StatusNoteKey string
	// Ended は、交換が終わったか (取り下げ・お断り・交換できた・交換できなかった・やめた)。終わった交換はこれまでの交換の中に置く。
	Ended bool
	// EndedDate は、交換が終わった日を、開いたユーザーのタイムゾーンで表示用に整えたもの。進行中の交換では空。
	EndedDate string
	// ReceiveHeadingKey・GiveHeadingKey は、もらう品・渡す品の見出しの翻訳キー。
	// 交換できた交換では「もらった・渡した」、ほかの終わった交換では「もらう予定だった・渡す予定だった」にする。
	// もらう品の見出しは相手の名前 (Name) を受け取る。
	ReceiveHeadingKey string
	GiveHeadingKey    string
	// CanWithdraw は、開いたユーザーが今申し込みを取り下げられるか。申し込んだ人で、返事待ちのときに限る。
	CanWithdraw bool
	// CanReply は、開いたユーザーが今申し込みに返事 (承認・お断り) ができるか。申し込まれた人で、返事待ちのときに限る。
	CanReply bool
	// CanComplete は、開いたユーザーが今「交換できた」を押せるか。マッチ成立で、まだ押していないときに限る。
	CanComplete bool
	// CanFail は、開いたユーザーが今「交換できなかった」を記録できるか。マッチ成立のときに限る。
	CanFail bool
	// CanCancel は、開いたユーザーが今交換をやめられるか。マッチ成立で、どちらも「交換できた」を押していないときに限る。
	CanCancel bool
	// PartnerCompleted は、相手が「交換できた」を押したか。押していれば、開いたユーザーが押すと交換が終わる。
	PartnerCompleted bool
	// MessageConsentValid は、開いたユーザーにメッセージの取り扱いへの有効な同意があるか。
	// 無ければ承認の代わりに同意の案内を出し、お断り・「交換できた」・「交換できなかった」ではひとことの欄を出さない。
	// ひとことが必須の交換をやめる画面では、フォームの代わりに同意の案内を出す。
	MessageConsentValid bool
	// LatestMessage は交換の最新のメッセージで、メッセージのページへの行に出す。メッセージが無ければnil。
	LatestMessage *TradeMessagePreview
	// UnreadMessageCount は開いたユーザーの未読のメッセージの数。
	UnreadMessageCount int64
}

// NewTradeDetail は、ユーザー viewerID が開いた交換 trade を交換のページに出す形にする。
//
// partner は交換の相手、items は交換の品のアイテムで、goods・categories には items のマスタをすべて入れて渡す。
// latestMessage は交換の最新のメッセージ (無ければnil)、unreadMessageCount は開いたユーザーの未読のメッセージの数。
// messageConsentValid は開いたユーザーにメッセージの取り扱いへの有効な同意があるか。
// 出来事の日は loc のタイムゾーンで、now と同じ年なら年を省いて示す。
func NewTradeDetail(
	ctx context.Context,
	viewerID model.UserID,
	trade *model.Trade,
	partner *model.User,
	items []*model.Item,
	goods map[model.GoodsID]*model.Goods,
	categories map[model.EventCategoryID]*model.EventCategory,
	events []*model.TradeEvent,
	latestMessage *model.TradeMessage,
	unreadMessageCount int64,
	messageConsentValid bool,
	loc *time.Location,
	now time.Time,
) TradeDetail {
	detail := TradeDetail{
		ID:                  trade.ID.String(),
		PartnerName:         tradePartnerName(ctx, partner),
		Status:              newTradeStatusBadge(trade, viewerID),
		CanWithdraw:         trade.Status == model.TradeStatusPending && trade.IsProposer(viewerID),
		CanReply:            trade.Status == model.TradeStatusPending && trade.ReceiverUserID == viewerID,
		CanComplete:         trade.Status == model.TradeStatusMatched && !trade.HasCompleted(viewerID),
		CanFail:             trade.Status == model.TradeStatusMatched,
		CanCancel:           trade.Status == model.TradeStatusMatched && !trade.HasAnyCompleted(),
		PartnerCompleted:    trade.HasCompleted(trade.PartnerUserID(viewerID)),
		MessageConsentValid: messageConsentValid,
		LatestMessage:       newTradeMessagePreview(ctx, viewerID, latestMessage, loc, now),
		UnreadMessageCount:  unreadMessageCount,
		Ended:               !trade.IsInProgress(),
		ReceiveHeadingKey:   "trade_show_receive_heading",
		GiveHeadingKey:      "trade_show_give_heading",
	}
	if partner.DeletedAt == nil {
		detail.PartnerAtname = partner.Atname
	}
	switch trade.Status {
	case model.TradeStatusCompleted:
		detail.ReceiveHeadingKey, detail.GiveHeadingKey = "trade_show_received_heading", "trade_show_gave_heading"
	case model.TradeStatusWithdrawn, model.TradeStatusDeclined, model.TradeStatusFailed, model.TradeStatusCancelled:
		detail.ReceiveHeadingKey, detail.GiveHeadingKey = "trade_show_would_receive_heading", "trade_show_would_give_heading"
	}
	if detail.Ended && trade.EndedAt != nil {
		detail.EndedDate = FormatDate(ctx, trade.EndedAt.In(loc), now.In(loc))
		detail.StatusNoteKey = endedTradeStatusNoteKey(trade, events, viewerID)
	}
	if trade.Status == model.TradeStatusMatched {
		switch {
		case trade.HasCompleted(viewerID):
			detail.StatusNoteKey = "trade_show_status_note_awaiting_partner_confirmation"
		case detail.PartnerCompleted:
			detail.StatusNoteKey = "trade_show_status_note_awaiting_your_confirmation"
		default:
			detail.StatusNoteKey = "trade_show_status_note_matched"
		}
	}
	for _, item := range items {
		row := newTradeItemRow(item, goods, categories)
		if item.UserID == viewerID {
			detail.Give = append(detail.Give, row)
		} else {
			detail.Receive = append(detail.Receive, row)
		}
	}
	for _, event := range events {
		keys := tradeEventTextKeys[event.Kind]
		row := TradeEventRow{Date: FormatDate(ctx, event.CreatedAt.In(loc), now.In(loc)), TextKey: keys.self, ReasonKey: tradeEventReasonKey(event)}
		if event.ActorUserID != viewerID {
			row.TextKey = keys.partner
			row.ActorName = detail.PartnerName
		}
		detail.Events = append(detail.Events, row)
	}

	return detail
}

// endedTradeEventKinds は、交換を終えた人が決まる段階と、その人を記録した出来事の種類。
// 交換できた (2人とも押した) と交換できなかった (どちらが記録したかを段階の文で問わない) は含めない。
var endedTradeEventKinds = map[model.TradeStatus]model.TradeEventKind{
	model.TradeStatusWithdrawn: model.TradeEventKindWithdrawn,
	model.TradeStatusDeclined:  model.TradeEventKindDeclined,
	model.TradeStatusCancelled: model.TradeEventKindCancelled,
}

// endedTradeStatusNoteKey は、終わった交換 trade の段階のバッジの横に添える、いつどう終わったかの文の翻訳キーを返す。
// 取り下げ・お断り・やめたは、交換を終えた出来事 events の起こした人が、開いたユーザー viewerID か相手かで文を分ける。
// 終えた出来事が見つからないときは空を返す。
func endedTradeStatusNoteKey(trade *model.Trade, events []*model.TradeEvent, viewerID model.UserID) string {
	switch trade.Status {
	case model.TradeStatusCompleted:
		return "trade_show_status_note_completed"
	case model.TradeStatusFailed:
		return "trade_show_status_note_failed"
	}

	kind, ok := endedTradeEventKinds[trade.Status]
	if !ok {
		return ""
	}
	for _, event := range slices.Backward(events) {
		if event.Kind != kind {
			continue
		}
		if event.ActorUserID == viewerID {
			return "trade_show_status_note_" + string(trade.Status) + "_self"
		}
		return "trade_show_status_note_" + string(trade.Status) + "_partner"
	}

	return ""
}
