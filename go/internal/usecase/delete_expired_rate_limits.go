package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/ratelimit"
)

// rateLimitRetention はレート制限のカウンターを残しておく期間。
//
// 判定に要るのは使っている時間枠のうち最長のもの (ログインの15分) までだが、
// 上限を超えた試行も数えているため、攻撃を受けたあとに規模を調べられるよう1日残す。
const rateLimitRetention = 24 * time.Hour

// DeleteExpiredRateLimitsUsecase は保持期間を過ぎたレート制限のカウンターを削除する。
// 定期ジョブから呼ばれ、利用者の入力を受け取らないため、バリデーターを持たない。
type DeleteExpiredRateLimitsUsecase struct {
	limiter *ratelimit.Limiter
}

// NewDeleteExpiredRateLimitsUsecase は DeleteExpiredRateLimitsUsecase を生成する。
func NewDeleteExpiredRateLimitsUsecase(limiter *ratelimit.Limiter) *DeleteExpiredRateLimitsUsecase {
	return &DeleteExpiredRateLimitsUsecase{limiter: limiter}
}

// Execute は保持期間より前に始まった時間枠のカウンターを削除する。
func (uc *DeleteExpiredRateLimitsUsecase) Execute(ctx context.Context) error {
	if err := uc.limiter.DeleteExpired(ctx, rateLimitRetention); err != nil {
		return fmt.Errorf("期限切れのレート制限のカウンターの削除に失敗: %w", err)
	}

	return nil
}
