package ratelimit

import (
	"context"
	"log/slog"
	"time"
)

// 交換のメッセージの送信の上限。固定ウィンドウで、受け付けなかった送信も1回と数える。
//
// メッセージは相手に届いて残るため、機械的に続けて送りつけるのを抑える。
// 送れるのはログイン中の本人だけのため、ユーザーの単位だけで数える。
// 会う場所と日時を決めるやり取りが続いても届かない、10分に30通 (20秒に1通) を上限にする。
const (
	tradeMessageAction    = "trade_message"
	tradeMessageWindow    = 10 * time.Minute
	tradeMessageUserLimit = 30
)

// CheckTradeMessage は、交換のメッセージの送信をユーザーの単位で数え、上限を超えていれば数え直しになる時刻を返す。
// 上限内ならゼロ値を返す。userID はユーザーのIDの文字列表記。
func (l *Limiter) CheckTradeMessage(ctx context.Context, userID string) (time.Time, error) {
	result, err := l.Check(ctx, CheckInput{
		Key:    UserKey(tradeMessageAction, userID),
		Limit:  tradeMessageUserLimit,
		Window: tradeMessageWindow,
	})
	if err != nil {
		return time.Time{}, err
	}
	if !result.Allowed {
		slog.WarnContext(ctx, "レート制限の上限を超えたため、交換のメッセージを受け付けません", "user_id", userID, "count", result.Count)
		return result.ResetAt, nil
	}

	return time.Time{}, nil
}
