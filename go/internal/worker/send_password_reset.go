package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// SendPasswordResetWorker はパスワードリセットのメールを送るジョブを処理する。
type SendPasswordResetWorker struct {
	river.WorkerDefaults[dispatcher.SendPasswordResetArgs]
	uc *usecase.SendPasswordResetUsecase
}

// NewSendPasswordResetWorker は SendPasswordResetWorker を生成する。
func NewSendPasswordResetWorker(uc *usecase.SendPasswordResetUsecase) *SendPasswordResetWorker {
	return &SendPasswordResetWorker{uc: uc}
}

// Work はリンクのトークンを発行し、パスワードリセットのメールを送る。
func (w *SendPasswordResetWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendPasswordResetArgs]) error {
	return w.uc.Execute(ctx, usecase.SendPasswordResetInput{
		UserID: job.Args.UserID,
		Locale: job.Args.Locale,
	})
}
