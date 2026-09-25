package ratelimit

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// ログインの二要素認証 (認証アプリのコードとリカバリーコード) の試行の上限。
// どちらも固定ウィンドウで、コードが合った試行も1回と数える。
//
// 2つの画面で同じアクション名のカウンターを使い、画面を行き来して試行の回数を増やせないようにする。
// ユーザーの上限は、パスワードを知られたアカウントへのコードの総当たりを抑える。
// 1回の試行で前後のタイムステップを含む3つのコードが通るため、回数は打ち間違いを数回許す程度に絞る。
// IPアドレスの上限は、同じ回線の複数の利用者を巻き込まないよう、ログインと同じ緩さにする。
const (
	signInTwoFactorAction    = "sign_in_two_factor"
	signInTwoFactorWindow    = 15 * time.Minute
	signInTwoFactorIPLimit   = 30
	signInTwoFactorUserLimit = 5
)

// CheckSignInTwoFactor は、ログインの二要素認証の試行をIPアドレスとユーザーの両方の単位で数え、
// どちらかが上限を超えていれば数え直しになる時刻を返す。どちらも上限内ならゼロ値を返す。
//
// 片方が超えていても両方を数える理由は、ログインと同じ (攻撃の規模を後から把握できるようにする)。
// userID はユーザーのIDの文字列表記。
func (l *Limiter) CheckSignInTwoFactor(ctx context.Context, ip, userID string) (time.Time, error) {
	checks := []struct {
		unit  string
		input CheckInput
	}{
		{unit: "ip", input: CheckInput{Key: IPKey(signInTwoFactorAction, ip), Limit: signInTwoFactorIPLimit, Window: signInTwoFactorWindow}},
		{unit: "user", input: CheckInput{Key: UserKey(signInTwoFactorAction, userID), Limit: signInTwoFactorUserLimit, Window: signInTwoFactorWindow}},
	}

	var resetAt time.Time
	for _, check := range checks {
		result, err := l.Check(ctx, check.input)
		if err != nil {
			return time.Time{}, err
		}
		if !result.Allowed {
			slog.WarnContext(ctx, "レート制限の上限を超えたため、ログインの二要素認証のコードを受け付けません", "unit", check.unit, "ip", ip, "user_id", userID, "count", result.Count)
			if result.ResetAt.After(resetAt) {
				resetAt = result.ResetAt
			}
		}
	}

	return resetAt, nil
}

// WaitMinutes は、数え直しになるまでの時間を分に切り上げて返す。
// 利用者に「あと○分」と示すための値で、解除の直前でも0分とは示さず1分以上にする。
func WaitMinutes(untilReset time.Duration) int {
	return max(int(math.Ceil(untilReset.Minutes())), 1)
}
