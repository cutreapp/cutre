package model

import "time"

// UserTwoFactorAuth はユーザーが認証アプリに登録したTOTPの設定。
//
// SecretCiphertext は暗号化したTOTPの秘密鍵で、平文へ戻すのは auth.TwoFactorKey だけが行う。
// LastUsedStep は最後に受け付けたコードのタイムステップで、0は一度も使っていないことを表す。
// EnabledAt は二要素認証を有効にした日時で、nilは認証アプリへの登録の途中を表す。
type UserTwoFactorAuth struct {
	ID               UserTwoFactorAuthID
	UserID           UserID
	SecretCiphertext []byte
	LastUsedStep     int64
	EnabledAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsEnabled は二要素認証を有効にしているかを返す。
// 登録の途中の設定では、ログインでコードを求めない。
func (a *UserTwoFactorAuth) IsEnabled() bool {
	return a.EnabledAt != nil
}
