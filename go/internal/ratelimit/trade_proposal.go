package ratelimit

import (
	"context"
	"log/slog"
	"time"
)

// 交換の申し込みの上限。固定ウィンドウで、受け付けなかった送信も1回と数える。
//
// 申し込みは相手に届き、ひとことはメッセージとして残るため、多くの相手へ続けて送りつけるのを抑える。
// 送れるのはログイン中の本人だけのため、ユーザーの単位だけで数える。
// 招待制の利用者が1時間に申し込む数としては十分に多く、機械的な連続の送信だけを止める値にする。
const (
	tradeProposalAction    = "trade_proposal"
	tradeProposalWindow    = time.Hour
	tradeProposalUserLimit = 10
)

// CheckTradeProposal は、交換の申し込みをユーザーの単位で数え、上限を超えていれば数え直しになる時刻を返す。
// 上限内ならゼロ値を返す。userID はユーザーのIDの文字列表記。
func (l *Limiter) CheckTradeProposal(ctx context.Context, userID string) (time.Time, error) {
	result, err := l.Check(ctx, CheckInput{
		Key:    UserKey(tradeProposalAction, userID),
		Limit:  tradeProposalUserLimit,
		Window: tradeProposalWindow,
	})
	if err != nil {
		return time.Time{}, err
	}
	if !result.Allowed {
		slog.WarnContext(ctx, "レート制限の上限を超えたため、交換の申し込みを受け付けません", "user_id", userID, "count", result.Count)
		return result.ResetAt, nil
	}

	return time.Time{}, nil
}
