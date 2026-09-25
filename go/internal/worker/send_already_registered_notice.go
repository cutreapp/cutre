package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// SendAlreadyRegisteredNoticeWorker は、登録済みのメールアドレスへログインを案内するメールを送るジョブを処理する。
type SendAlreadyRegisteredNoticeWorker struct {
	river.WorkerDefaults[dispatcher.SendAlreadyRegisteredNoticeArgs]
	uc *usecase.SendAlreadyRegisteredNoticeUsecase
}

// NewSendAlreadyRegisteredNoticeWorker は SendAlreadyRegisteredNoticeWorker を生成する。
func NewSendAlreadyRegisteredNoticeWorker(uc *usecase.SendAlreadyRegisteredNoticeUsecase) *SendAlreadyRegisteredNoticeWorker {
	return &SendAlreadyRegisteredNoticeWorker{uc: uc}
}

// Work はログインを案内するメールを送る。
func (w *SendAlreadyRegisteredNoticeWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendAlreadyRegisteredNoticeArgs]) error {
	return w.uc.Execute(ctx, usecase.SendAlreadyRegisteredNoticeInput{
		Email:  job.Args.Email,
		Locale: job.Args.Locale,

		IdempotencyKey: idempotencyKey(job.JobRow),
	})
}
