package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// DeleteExpiredEmailConfirmationsWorker は使われずに残ったメールアドレスの確認を削除するジョブを処理する。
type DeleteExpiredEmailConfirmationsWorker struct {
	river.WorkerDefaults[dispatcher.DeleteExpiredEmailConfirmationsArgs]
	uc *usecase.DeleteExpiredEmailConfirmationsUsecase
}

// NewDeleteExpiredEmailConfirmationsWorker は DeleteExpiredEmailConfirmationsWorker を生成する。
func NewDeleteExpiredEmailConfirmationsWorker(uc *usecase.DeleteExpiredEmailConfirmationsUsecase) *DeleteExpiredEmailConfirmationsWorker {
	return &DeleteExpiredEmailConfirmationsWorker{uc: uc}
}

// Work は保持期間を過ぎたメールアドレスの確認を削除する。
func (w *DeleteExpiredEmailConfirmationsWorker) Work(ctx context.Context, _ *river.Job[dispatcher.DeleteExpiredEmailConfirmationsArgs]) error {
	return w.uc.Execute(ctx)
}
