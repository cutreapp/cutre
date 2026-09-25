package model

import "time"

// InvitationRedemption は招待の使用の記録。1件が「この招待でこのユーザーが登録した」を表す。
// 退会後も招待の経路を辿れるよう、招待で登録した人が退会しても残す。
type InvitationRedemption struct {
	ID           InvitationRedemptionID
	InvitationID InvitationID
	UserID       UserID
	// User は招待で登録したユーザー。登録した人と合わせて引いたときだけ持ち、それ以外はnil。
	User      *User
	CreatedAt time.Time
	UpdatedAt time.Time
}
