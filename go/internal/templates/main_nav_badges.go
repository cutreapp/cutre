package templates

import "context"

// MainNavBadges はログイン後のページのメインメニューの項目に出す数字。
type MainNavBadges struct {
	// AwaitingTradeCount は、ログイン中のユーザーの返事や確認を待っている交換の数。交換の項目に出す。
	AwaitingTradeCount int64
	// UnreadMessageCount は、ログイン中のユーザーの未読のメッセージの数。メッセージの項目に出す。
	UnreadMessageCount int64
}

// mainNavBadgesContextKey はメインメニューの数字をリクエストcontextに載せるときのキー。
type mainNavBadgesContextKey struct{}

// WithMainNavBadges はメインメニューの数字を載せたcontextを返す。
// ログイン後のページのすべてで同じ数字を出すため、ハンドラーごとに引かず、ミドルウェアが1か所で載せる。
func WithMainNavBadges(ctx context.Context, badges MainNavBadges) context.Context {
	return context.WithValue(ctx, mainNavBadgesContextKey{}, badges)
}

// MainNavBadgesFromContext はcontextに載ったメインメニューの数字を返す。載っていなければゼロ値 (数字を出さない) を返す。
func MainNavBadgesFromContext(ctx context.Context) MainNavBadges {
	badges, _ := ctx.Value(mainNavBadgesContextKey{}).(MainNavBadges)
	return badges
}
