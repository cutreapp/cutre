// Package model はドメインエンティティと、それを構成する値型を提供する。
package model

import "github.com/google/uuid"

// UserID はユーザーの識別子。
//
// データベースが採番する uuid をそのまま使わずラップするのは、
// 別のエンティティのIDを取り違えて渡すとコンパイルエラーになるようにするため。
type UserID uuid.UUID

// String はUserIDをUUIDの文字列表記で返す。
func (id UserID) String() string { return uuid.UUID(id).String() }

// MarshalText はUserIDをUUIDの文字列表記で書き出す。
// ジョブの引数のようにJSONへ載せたとき、16個の数値の配列ではなく読めるUUIDにするため。
func (id UserID) MarshalText() ([]byte, error) { return uuid.UUID(id).MarshalText() }

// UnmarshalText はUUIDの文字列表記からUserIDを読み取る。
func (id *UserID) UnmarshalText(text []byte) error { return (*uuid.UUID)(id).UnmarshalText(text) }

// UserPasswordID はパスワード資格情報の識別子。UserIDと同じ理由でuuidをラップする。
type UserPasswordID uuid.UUID

// String はUserPasswordIDをUUIDの文字列表記で返す。
func (id UserPasswordID) String() string { return uuid.UUID(id).String() }

// UserSessionID はログイン中のセッションの識別子。UserIDと同じ理由でuuidをラップする。
type UserSessionID uuid.UUID

// String はUserSessionIDをUUIDの文字列表記で返す。
func (id UserSessionID) String() string { return uuid.UUID(id).String() }

// RateLimitID はレート制限のカウンターの識別子。UserIDと同じ理由でuuidをラップする。
type RateLimitID uuid.UUID

// String はRateLimitIDをUUIDの文字列表記で返す。
func (id RateLimitID) String() string { return uuid.UUID(id).String() }

// InvitationID は招待の識別子。UserIDと同じ理由でuuidをラップする。
type InvitationID uuid.UUID

// String はInvitationIDをUUIDの文字列表記で返す。
func (id InvitationID) String() string { return uuid.UUID(id).String() }

// InvitationRedemptionID は招待の使用の記録の識別子。UserIDと同じ理由でuuidをラップする。
type InvitationRedemptionID uuid.UUID

// String はInvitationRedemptionIDをUUIDの文字列表記で返す。
func (id InvitationRedemptionID) String() string { return uuid.UUID(id).String() }

// EmailConfirmationID はメールアドレスの確認の識別子。UserIDと同じ理由でuuidをラップする。
type EmailConfirmationID uuid.UUID

// String はEmailConfirmationIDをUUIDの文字列表記で返す。
func (id EmailConfirmationID) String() string { return uuid.UUID(id).String() }

// PasswordResetTokenID はパスワードリセットのトークンの識別子。UserIDと同じ理由でuuidをラップする。
type PasswordResetTokenID uuid.UUID

// String はPasswordResetTokenIDをUUIDの文字列表記で返す。
func (id PasswordResetTokenID) String() string { return uuid.UUID(id).String() }

// UserTwoFactorAuthID は二要素認証の設定の識別子。UserIDと同じ理由でuuidをラップする。
type UserTwoFactorAuthID uuid.UUID

// String はUserTwoFactorAuthIDをUUIDの文字列表記で返す。
func (id UserTwoFactorAuthID) String() string { return uuid.UUID(id).String() }
