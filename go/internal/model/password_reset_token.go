package model

import "time"

// PasswordResetTokenLifetime はパスワードリセットのリンクが使える期間。
const PasswordResetTokenLifetime = time.Hour

// PasswordResetTokenExpiresAt はnowに発行したパスワードリセットのトークンの有効期限を返す。
func PasswordResetTokenExpiresAt(now time.Time) time.Time {
	return now.Add(PasswordResetTokenLifetime)
}

// PasswordResetToken はパスワードリセットのメールに載せたリンクの使い捨てトークン。
//
// TokenDigest はリンクが運ぶトークンのダイジェストで、平文のトークンはメールの中にしか無い。
type PasswordResetToken struct {
	ID          PasswordResetTokenID
	UserID      UserID
	TokenDigest string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
