package model

import "time"

// UserPassword はユーザーのパスワード資格情報。
// PasswordDigest はbcryptハッシュで、平文は保持しない。
// 1ユーザーが持つUserPasswordは高々1つ (user_idのUNIQUE制約で保証する)。
type UserPassword struct {
	ID             UserPasswordID
	UserID         UserID
	PasswordDigest string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
