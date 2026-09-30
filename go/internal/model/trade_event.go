package model

import "time"

// TradeEventKind は交換の出来事の種類。
type TradeEventKind string

const (
	// TradeEventKindProposed は申し込み。
	TradeEventKindProposed TradeEventKind = "proposed"
	// TradeEventKindWithdrawn は申し込みの取り下げ。
	TradeEventKindWithdrawn TradeEventKind = "withdrawn"
	// TradeEventKindApproved は承認。
	TradeEventKindApproved TradeEventKind = "approved"
	// TradeEventKindDeclined はお断り。
	TradeEventKindDeclined TradeEventKind = "declined"
	// TradeEventKindCompleted は、2人のうち1人が「交換できた」を押したこと。
	TradeEventKindCompleted TradeEventKind = "completed"
	// TradeEventKindFailed は「交換できなかった」の記録。
	TradeEventKindFailed TradeEventKind = "failed"
	// TradeEventKindCancelled は交換をやめたこと。
	TradeEventKindCancelled TradeEventKind = "cancelled"
)

// TradeEvent は交換の出来事。交換のページの「これまでの流れ」と、メッセージの流れに入るお知らせに使う。
//
// ActorUserID はその出来事を起こしたユーザー。
// Reason は、お断り・交換できなかった・やめたで選んだ理由の選択肢のキーで、理由を選ばない出来事ではnil。
type TradeEvent struct {
	ID          TradeEventID
	TradeID     TradeID
	ActorUserID UserID
	Kind        TradeEventKind
	Reason      *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TradeDeclineReason はお断りで選ぶ理由の選択肢のキー。TradeEvent.Reason に入れる。
// 選択肢は文言と一緒に変わりやすいため、データベースではENUMにせず文字列で持ち、許す値はバリデーターで決める。
type TradeDeclineReason string

const (
	// TradeDeclineReasonAlreadyDecided は、もう交換が決まった。
	TradeDeclineReasonAlreadyDecided TradeDeclineReason = "already_decided"
	// TradeDeclineReasonPlaceMismatch は、交換場所が合わない。
	TradeDeclineReasonPlaceMismatch TradeDeclineReason = "place_mismatch"
	// TradeDeclineReasonOtherItems は、ほかの品と交換したい。
	TradeDeclineReasonOtherItems TradeDeclineReason = "other_items"
	// TradeDeclineReasonOther は、その他。
	TradeDeclineReasonOther TradeDeclineReason = "other"
)

// TradeDeclineReasons はお断りの理由の選択肢を、画面に並べる順で返す。
func TradeDeclineReasons() []TradeDeclineReason {
	return []TradeDeclineReason{
		TradeDeclineReasonAlreadyDecided,
		TradeDeclineReasonPlaceMismatch,
		TradeDeclineReasonOtherItems,
		TradeDeclineReasonOther,
	}
}

// ParseTradeDeclineReason は文字列をお断りの理由の選択肢として読む。選択肢に無ければfalseを返す。
func ParseTradeDeclineReason(s string) (TradeDeclineReason, bool) {
	return parseTradeReason(s, TradeDeclineReasons())
}

// TradeFailureReason は「交換できなかった」で選ぶ理由の選択肢のキー。TradeEvent.Reason に入れる。
// お断りの理由と同じく、データベースではENUMにせず文字列で持つ。
type TradeFailureReason string

const (
	// TradeFailureReasonNoShow は、当日会えなかった。
	TradeFailureReasonNoShow TradeFailureReason = "no_show"
	// TradeFailureReasonNotExchanged は、会えたが、交換しなかった。
	TradeFailureReasonNotExchanged TradeFailureReason = "not_exchanged"
	// TradeFailureReasonUnavailable は、都合が悪くなった。
	TradeFailureReasonUnavailable TradeFailureReason = "unavailable"
	// TradeFailureReasonLostContact は、連絡が取れなくなった。
	TradeFailureReasonLostContact TradeFailureReason = "lost_contact"
	// TradeFailureReasonOther は、その他。
	TradeFailureReasonOther TradeFailureReason = "other"
)

// TradeFailureReasons は「交換できなかった」の理由の選択肢を、画面に並べる順で返す。
func TradeFailureReasons() []TradeFailureReason {
	return []TradeFailureReason{
		TradeFailureReasonNoShow,
		TradeFailureReasonNotExchanged,
		TradeFailureReasonUnavailable,
		TradeFailureReasonLostContact,
		TradeFailureReasonOther,
	}
}

// ParseTradeFailureReason は文字列を「交換できなかった」の理由の選択肢として読む。選択肢に無ければfalseを返す。
func ParseTradeFailureReason(s string) (TradeFailureReason, bool) {
	return parseTradeReason(s, TradeFailureReasons())
}

// TradeCancellationReason は交換をやめるときに選ぶ理由の選択肢のキー。TradeEvent.Reason に入れる。
// お断りの理由と同じく、データベースではENUMにせず文字列で持つ。
type TradeCancellationReason string

const (
	// TradeCancellationReasonDecidedElsewhere は、ほかの人と交換が決まった。
	TradeCancellationReasonDecidedElsewhere TradeCancellationReason = "decided_elsewhere"
	// TradeCancellationReasonScheduleMismatch は、予定が合わなくなった。
	TradeCancellationReasonScheduleMismatch TradeCancellationReason = "schedule_mismatch"
	// TradeCancellationReasonItemUnavailable は、品物が手元になくなった。
	TradeCancellationReasonItemUnavailable TradeCancellationReason = "item_unavailable"
	// TradeCancellationReasonOther は、その他。
	TradeCancellationReasonOther TradeCancellationReason = "other"
)

// TradeCancellationReasons は交換をやめる理由の選択肢を、画面に並べる順で返す。
func TradeCancellationReasons() []TradeCancellationReason {
	return []TradeCancellationReason{
		TradeCancellationReasonDecidedElsewhere,
		TradeCancellationReasonScheduleMismatch,
		TradeCancellationReasonItemUnavailable,
		TradeCancellationReasonOther,
	}
}

// ParseTradeCancellationReason は文字列を交換をやめる理由の選択肢として読む。選択肢に無ければfalseを返す。
func ParseTradeCancellationReason(s string) (TradeCancellationReason, bool) {
	return parseTradeReason(s, TradeCancellationReasons())
}

// parseTradeReason は文字列を理由の選択肢 choices のどれかとして読む。選択肢に無ければfalseを返す。
func parseTradeReason[T ~string](s string, choices []T) (T, bool) {
	for _, reason := range choices {
		if string(reason) == s {
			return reason, true
		}
	}

	var zero T
	return zero, false
}
