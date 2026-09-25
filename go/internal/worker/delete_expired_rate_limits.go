package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// DeleteExpiredRateLimitsWorker は保持期間を過ぎたレート制限のカウンターを削除するジョブを処理する。
type DeleteExpiredRateLimitsWorker struct {
	river.WorkerDefaults[dispatcher.DeleteExpiredRateLimitsArgs]
	uc *usecase.DeleteExpiredRateLimitsUsecase
}

// NewDeleteExpiredRateLimitsWorker は DeleteExpiredRateLimitsWorker を生成する。
func NewDeleteExpiredRateLimitsWorker(uc *usecase.DeleteExpiredRateLimitsUsecase) *DeleteExpiredRateLimitsWorker {
	return &DeleteExpiredRateLimitsWorker{uc: uc}
}

// Work は保持期間を過ぎたレート制限のカウンターを削除する。
func (w *DeleteExpiredRateLimitsWorker) Work(ctx context.Context, _ *river.Job[dispatcher.DeleteExpiredRateLimitsArgs]) error {
	return w.uc.Execute(ctx)
}
