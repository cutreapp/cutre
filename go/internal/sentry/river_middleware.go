package sentry

import (
	"context"
	"strconv"

	"github.com/getsentry/sentry-go"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// RiverWorkerMiddleware はワーカーが最後に返したエラーをSentryのイベントとして送るRiverのミドルウェアを返す。
//
// ジョブごとに複製したHubの上で動かすため、ジョブのタグ (job.kind / job.attempt) はほかのジョブに漏れない。
// 複製したHubはcontextにも載せ、ジョブの中の slog.ErrorContext も同じスコープでイベントにする。
//
// shouldDropError に当たるエラーは、Riverの再試行の判断を変えないようそのまま返すが、Sentryには送らない。
// シャットダウンに由来する中断を誤って通知しないため。
func RiverWorkerMiddleware() rivertype.WorkerMiddleware {
	return river.WorkerMiddlewareFunc(func(ctx context.Context, job *rivertype.JobRow, doInner func(ctx context.Context) error) error {
		hub := cloneHubForJob(ctx)
		hub.Scope().SetTag("job.kind", job.Kind)
		hub.Scope().SetTag("job.attempt", strconv.Itoa(job.Attempt))
		ctx = sentry.SetHubOnContext(ctx, hub)

		err := doInner(ctx)
		if err != nil && !shouldDropError(err) {
			hub.CaptureException(err)
		}
		return err
	})
}

// cloneHubForJob はジョブで使うHubを複製して返す。
// contextにHubが載っていればそれを、無ければグローバルのHubを複製し、ジョブのスコープの変更を元のHubへ波及させない。
func cloneHubForJob(ctx context.Context) *sentry.Hub {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		return hub.Clone()
	}
	return sentry.CurrentHub().Clone()
}
