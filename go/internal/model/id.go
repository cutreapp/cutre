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

// MessageConsentID はメッセージの取り扱いへの同意の識別子。UserIDと同じ理由でuuidをラップする。
type MessageConsentID uuid.UUID

// String はMessageConsentIDをUUIDの文字列表記で返す。
func (id MessageConsentID) String() string { return uuid.UUID(id).String() }

// EventID はイベントの識別子。UserIDと同じ理由でuuidをラップする。
type EventID uuid.UUID

// String はEventIDをUUIDの文字列表記で返す。
func (id EventID) String() string { return uuid.UUID(id).String() }

// EventCategoryID はカテゴリーの識別子。UserIDと同じ理由でuuidをラップする。
type EventCategoryID uuid.UUID

// String はEventCategoryIDをUUIDの文字列表記で返す。
func (id EventCategoryID) String() string { return uuid.UUID(id).String() }

// GoodsID はグッズの識別子。UserIDと同じ理由でuuidをラップする。
type GoodsID uuid.UUID

// String はGoodsIDをUUIDの文字列表記で返す。
func (id GoodsID) String() string { return uuid.UUID(id).String() }

// StationID は駅の識別子。UserIDと同じ理由でuuidをラップする。
type StationID uuid.UUID

// String はStationIDをUUIDの文字列表記で返す。
func (id StationID) String() string { return uuid.UUID(id).String() }

// ItemID はリストのアイテムの識別子。UserIDと同じ理由でuuidをラップする。
type ItemID uuid.UUID

// String はItemIDをUUIDの文字列表記で返す。
func (id ItemID) String() string { return uuid.UUID(id).String() }

// TradeID は交換の識別子。UserIDと同じ理由でuuidをラップする。
type TradeID uuid.UUID

// String はTradeIDをUUIDの文字列表記で返す。
func (id TradeID) String() string { return uuid.UUID(id).String() }

// TradeEventID は交換の出来事の識別子。UserIDと同じ理由でuuidをラップする。
type TradeEventID uuid.UUID

// String はTradeEventIDをUUIDの文字列表記で返す。
func (id TradeEventID) String() string { return uuid.UUID(id).String() }

// TradeMessageID は交換のメッセージの識別子。UserIDと同じ理由でuuidをラップする。
type TradeMessageID uuid.UUID

// String はTradeMessageIDをUUIDの文字列表記で返す。
func (id TradeMessageID) String() string { return uuid.UUID(id).String() }
