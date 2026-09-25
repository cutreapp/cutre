package ratelimit

import (
	"context"
	"log/slog"
	"time"
)

// 二要素認証を無効にするときの再認証 (パスワードまたは認証アプリのコード) の試行の上限。
// 固定ウィンドウで、入力が合った試行も1回と数える。
//
// ログイン済みのセッションを奪った人が、パスワードやコードを総当たりして二要素認証を外すのを抑える。
// 送れるのはログイン中の本人だけのため、ユーザーの単位だけで数え、上限はログインの二要素認証に揃える。
const (
	twoFactorAuthDisableAction    = "two_factor_auth_disable"
	twoFactorAuthDisableWindow    = 15 * time.Minute
	twoFactorAuthDisableUserLimit = 5
)

// CheckTwoFactorAuthDisable は、二要素認証を無効にするときの再認証の試行をユーザーの単位で数え、
// 上限を超えていれば数え直しになる時刻を返す。上限内ならゼロ値を返す。
// userID はユーザーのIDの文字列表記。
func (l *Limiter) CheckTwoFactorAuthDisable(ctx context.Context, userID string) (time.Time, error) {
	result, err := l.Check(ctx, CheckInput{
		Key:    UserKey(twoFactorAuthDisableAction, userID),
		Limit:  twoFactorAuthDisableUserLimit,
		Window: twoFactorAuthDisableWindow,
	})
	if err != nil {
		return time.Time{}, err
	}
	if !result.Allowed {
		slog.WarnContext(ctx, "レート制限の上限を超えたため、二要素認証の無効化の再認証を受け付けません", "user_id", userID, "count", result.Count)
		return result.ResetAt, nil
	}

	return time.Time{}, nil
}
