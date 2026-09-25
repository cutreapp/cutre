package ratelimit

import (
	"context"
	"log/slog"
	"time"
)

// 退会するときの今のパスワードの試行の上限。
// 固定ウィンドウで、パスワードが合った試行も1回と数える。
//
// ログイン済みのセッションを奪った人が、パスワードを総当たりして当てたり、退会させたりするのを抑える。
// 送れるのはログイン中の本人だけのため、ユーザーの単位だけで数え、上限は二要素認証の無効化に揃える。
const (
	withdrawalAction    = "withdrawal"
	withdrawalWindow    = 15 * time.Minute
	withdrawalUserLimit = 5
)

// CheckWithdrawal は、退会するときのパスワードの試行をユーザーの単位で数え、
// 上限を超えていれば数え直しになる時刻を返す。上限内ならゼロ値を返す。
// userID はユーザーのIDの文字列表記。
func (l *Limiter) CheckWithdrawal(ctx context.Context, userID string) (time.Time, error) {
	result, err := l.Check(ctx, CheckInput{
		Key:    UserKey(withdrawalAction, userID),
		Limit:  withdrawalUserLimit,
		Window: withdrawalWindow,
	})
	if err != nil {
		return time.Time{}, err
	}
	if !result.Allowed {
		slog.WarnContext(ctx, "レート制限の上限を超えたため、退会のパスワードの確認を受け付けません", "user_id", userID, "count", result.Count)
		return result.ResetAt, nil
	}

	return time.Time{}, nil
}
