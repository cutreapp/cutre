package model

import "time"

// TradeMessage は、交換の2人が交換ごとに送り合うメッセージ。申し込みのひとことが1通目になる。
//
// RetractedAt は送った人が取り消した日時で、nilは取り消していないことを表す。
// 取り消しても Body は消さずに残す。利用者から問題の報告を受けたときに、運営が確認できるようにするため。
type TradeMessage struct {
	ID           TradeMessageID
	TradeID      TradeID
	SenderUserID UserID
	Body         string
	RetractedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TradeMessageReadPosition は、表示済みの最後のメッセージを送信時刻とIDの組で表す。
// 同じ時刻のメッセージも表示順に区別する。
type TradeMessageReadPosition struct {
	CreatedAt time.Time
	MessageID TradeMessageID
}
