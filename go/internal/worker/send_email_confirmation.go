package worker

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// SendEmailConfirmationWorker は確認コードのメールを送るジョブを処理する。
type SendEmailConfirmationWorker struct {
	river.WorkerDefaults[dispatcher.SendEmailConfirmationArgs]
	uc *usecase.SendEmailConfirmationUsecase
}

// NewSendEmailConfirmationWorker は SendEmailConfirmationWorker を生成する。
func NewSendEmailConfirmationWorker(uc *usecase.SendEmailConfirmationUsecase) *SendEmailConfirmationWorker {
	return &SendEmailConfirmationWorker{uc: uc}
}

// Work は確認コードのメールを送る。
func (w *SendEmailConfirmationWorker) Work(ctx context.Context, job *river.Job[dispatcher.SendEmailConfirmationArgs]) error {
	return w.uc.Execute(ctx, usecase.SendEmailConfirmationInput{
		Email:  job.Args.Email,
		Code:   job.Args.Code,
		Locale: job.Args.Locale,

		IdempotencyKey: idempotencyKey(job.JobRow),
	})
}
