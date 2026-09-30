package model

import "time"

// TradeStatus は交換の段階。
type TradeStatus string

const (
	// TradeStatusPending は、申し込まれた人の返事を待っている段階。
	TradeStatusPending TradeStatus = "pending"
	// TradeStatusWithdrawn は、返事の前に申し込んだ人が取り下げて終わった段階。
	TradeStatusWithdrawn TradeStatus = "withdrawn"
	// TradeStatusDeclined は、申し込まれた人がお断りして終わった段階。
	TradeStatusDeclined TradeStatus = "declined"
	// TradeStatusMatched は、申し込まれた人が承認し、2人が会って交換するのを待っている段階 (マッチ成立)。
	TradeStatusMatched TradeStatus = "matched"
	// TradeStatusCompleted は、2人とも「交換できた」を押して終わった段階。
	TradeStatusCompleted TradeStatus = "completed"
	// TradeStatusFailed は、どちらかが「交換できなかった」を記録して終わった段階。
	TradeStatusFailed TradeStatus = "failed"
	// TradeStatusCancelled は、どちらかが交換をやめて終わった段階。
	TradeStatusCancelled TradeStatus = "cancelled"
)

// Trade は、ユーザー (申し込んだ人) がほかのユーザー (申し込まれた人) に申し込んだ交換。
//
// 片方だけが「交換できた」を押した状態は段階を増やさず、ProposerCompletedAt と ReceiverCompletedAt の片方だけが入っていることで表す。
// MatchedAt は承認した日時、EndedAt は交換が終わった (取り下げ・お断りを含む) 日時で、それまではnil。
type Trade struct {
	ID                  TradeID
	ProposerUserID      UserID
	ReceiverUserID      UserID
	Status              TradeStatus
	ProposerCompletedAt *time.Time
	ReceiverCompletedAt *time.Time
	MatchedAt           *time.Time
	EndedAt             *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// IsInProgress は、交換が進行中 (返事待ちかマッチ成立) であるかを返す。進行中の交換では、2人がメッセージを送り合える。
func (t *Trade) IsInProgress() bool {
	return t.Status == TradeStatusPending || t.Status == TradeStatusMatched
}

// IsParticipant は、ユーザー userID が交換の2人 (申し込んだ人か申し込まれた人) のどちらかであるかを返す。
func (t *Trade) IsParticipant(userID UserID) bool {
	return t.ProposerUserID == userID || t.ReceiverUserID == userID
}

// IsProposer は、ユーザー userID が交換を申し込んだ人であるかを返す。
func (t *Trade) IsProposer(userID UserID) bool {
	return t.ProposerUserID == userID
}

// PartnerUserID は、交換の2人のうちユーザー userID の相手を返す。userID は交換の2人のどちらかを渡す。
func (t *Trade) PartnerUserID(userID UserID) UserID {
	if t.ProposerUserID == userID {
		return t.ReceiverUserID
	}

	return t.ProposerUserID
}

// HasCompleted は、ユーザー userID が「交換できた」を押したかを返す。userID は交換の2人のどちらかを渡す。
func (t *Trade) HasCompleted(userID UserID) bool {
	if t.ProposerUserID == userID {
		return t.ProposerCompletedAt != nil
	}

	return t.ReceiverCompletedAt != nil
}

// HasAnyCompleted は、2人のどちらかが「交換できた」を押したかを返す。どちらかが押した交換は、やめられない。
func (t *Trade) HasAnyCompleted() bool {
	return t.ProposerCompletedAt != nil || t.ReceiverCompletedAt != nil
}

// IsAwaiting は、交換がユーザー userID の返事や確認を待っているかを返す。
//
// 申し込まれて返事をしていない交換と、マッチ成立のあとに相手だけが「交換できた」を押した交換が当たる。
// メインメニューの交換の数字と同じ条件で、数えるクエリ (CountAwaitingTradesByUserID) と揃える。
func (t *Trade) IsAwaiting(userID UserID) bool {
	switch t.Status {
	case TradeStatusPending:
		return t.ReceiverUserID == userID
	case TradeStatusMatched:
		if t.ProposerUserID == userID {
			return t.ProposerCompletedAt == nil && t.ReceiverCompletedAt != nil
		}
		if t.ReceiverUserID == userID {
			return t.ReceiverCompletedAt == nil && t.ProposerCompletedAt != nil
		}
	}

	return false
}
