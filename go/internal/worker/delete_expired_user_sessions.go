package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// DeleteExpiredUserSessionsWorker は期限切れのセッションを削除するジョブを処理する。
type DeleteExpiredUserSessionsWorker struct {
	river.WorkerDefaults[dispatcher.DeleteExpiredUserSessionsArgs]
	uc *usecase.DeleteExpiredUserSessionsUsecase
}

// NewDeleteExpiredUserSessionsWorker は DeleteExpiredUserSessionsWorker を生成する。
func NewDeleteExpiredUserSessionsWorker(uc *usecase.DeleteExpiredUserSessionsUsecase) *DeleteExpiredUserSessionsWorker {
	return &DeleteExpiredUserSessionsWorker{uc: uc}
}

// Work は期限切れのセッションを削除する。
func (w *DeleteExpiredUserSessionsWorker) Work(ctx context.Context, _ *river.Job[dispatcher.DeleteExpiredUserSessionsArgs]) error {
	return w.uc.Execute(ctx)
}
