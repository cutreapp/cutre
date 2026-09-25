package model

import "time"

// UserSessionLifetime はログイン中のセッションの有効期間。
// この長さをログイン時とアクセスによる延長の両方で使うため、期限は常に「最後に使った時刻 + この長さ」になる。
const UserSessionLifetime = 30 * 24 * time.Hour

// userSessionExtensionInterval はセッションの期限を延長する間隔。
//
// 有効期限を延ばすUPDATEはリクエストごとに走らせる必要がない。
// 最後に使ってからこの時間が経つまでは延長を省き、閲覧のたびに書き込みが起きないようにする。
// 間引いても期限が縮むことはない。延長を省くのは期限がまだ29日以上残っているときだけである。
const userSessionExtensionInterval = 24 * time.Hour

// UserSession はCookieに紐づくログイン中のセッション。
//
// TokenDigest はCookieが運ぶトークンのダイジェストで、平文のトークンはCookieの中にしか無い。
// ExpiresAt はサーバー側の有効期限、LastSeenAt は最後にセッションを使った時刻、
// SignedInAt はログインした時刻で、IPAddress / UserAgent はログインした端末を記録する。
//
// User はセッションの持ち主。まとめて引いたときに設定され、そうでなければnilになる。
type UserSession struct {
	ID          UserSessionID
	UserID      UserID
	TokenDigest string
	ExpiresAt   time.Time
	LastSeenAt  time.Time
	IPAddress   string
	UserAgent   string
	SignedInAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	User *User
}

// NeedsExtension はnowの時点でセッションの期限を延長すべきかを返す。
func (s *UserSession) NeedsExtension(now time.Time) bool {
	return !now.Before(s.LastSeenAt.Add(userSessionExtensionInterval))
}

// UserSessionExpiresAt はnowを起点にしたセッションの有効期限を返す。
// ログイン時の期限も延長後の期限もここで決め、両者が別々の長さにならないようにする。
func UserSessionExpiresAt(now time.Time) time.Time {
	return now.Add(UserSessionLifetime)
}
