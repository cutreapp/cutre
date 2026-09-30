package policy

import "github.com/cutreapp/cutre/go/internal/model"

// TradePolicy は交換の操作をしてよいかを、操作するユーザーが交換の2人のどちらかで判定する。
type TradePolicy struct {
	userID model.UserID
	trade  *model.Trade
}

// NewTradePolicy はユーザー userID が交換 trade を操作するときの TradePolicy を生成する。
func NewTradePolicy(userID model.UserID, trade *model.Trade) *TradePolicy {
	return &TradePolicy{userID: userID, trade: trade}
}

// CanView は交換のページを見られるかを返す。交換とそのやり取りは2人だけのものため、2人に許す。
func (p *TradePolicy) CanView() bool {
	return p.trade.IsParticipant(p.userID)
}

// CanWithdraw は申し込みを取り下げられるかを返す。申し込んだ人に許す。
// 返事の前かは段階が変わりうるため、ここでは見ず、取り下げの更新の条件で確かめる。
func (p *TradePolicy) CanWithdraw() bool {
	return p.trade.IsProposer(p.userID)
}

// CanReply は申し込みに返事 (承認・お断り) ができるかを返す。申し込まれた人に許す。
// 返事の前かは段階が変わりうるため、ここでは見ず、承認・お断りの更新の条件で確かめる。
func (p *TradePolicy) CanReply() bool {
	return p.trade.ReceiverUserID == p.userID
}

// CanRetractMessage はメッセージ message を取り消せるかを返す。この交換のメッセージで、送った人に許す。
// 送ったメッセージはいつでも取り消せるため、交換の段階は問わない。
func (p *TradePolicy) CanRetractMessage(message *model.TradeMessage) bool {
	return p.CanView() && message.TradeID == p.trade.ID && message.SenderUserID == p.userID
}
