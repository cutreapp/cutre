package model

import "time"

// CurrentMessageConsentVersion は、メッセージの取り扱いの今の文面の版。
//
// 文面は翻訳ファイルの message_consent_term_* のキーが持つ。
// 文面の意味を変えたら版を上げ、それより前の版の同意を有効として扱わないようにする。
// 誤字の修正のように、同意した内容が変わらない直しでは上げない。
const CurrentMessageConsentVersion = 1

// MessageConsent はメッセージの取り扱いへの同意の記録。
//
// Version は同意した文面の版、AgreedAt は同意した日時。
// WithdrawnAt は同意をやめた日時で、nilはやめていないことを表す。
// やめてからもう一度同意したときは、新しい記録を作る。
type MessageConsent struct {
	ID          MessageConsentID
	UserID      UserID
	Version     int
	AgreedAt    time.Time
	WithdrawnAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsValid は、この同意で交換の申し込みと承認ができるかを返す。
// 今の版の文面に同意していて、やめていないときに限る。
// ユーザーの最新の記録に対して呼ぶ。古い記録が有効でも、後の記録でやめていれば同意は無い。
func (c *MessageConsent) IsValid() bool {
	return c.Version == CurrentMessageConsentVersion && c.WithdrawnAt == nil
}
