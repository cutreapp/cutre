package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/cutreapp/cutre/go/internal/dispatcher"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/worker"
)

// TestDeleteExpiredRateLimitsWorker_Work は、ジョブの処理で保持期間を過ぎたカウンターが消えることを検証する。
func TestDeleteExpiredRateLimitsWorker_Work(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewRateLimitRepository(db).WithTx(tx)
	w := worker.NewDeleteExpiredRateLimitsWorker(usecase.NewDeleteExpiredRateLimitsUsecase(ratelimit.NewLimiter(repo)))

	key := "test:" + testutil.UniqueAtname()
	expired := time.Now().UTC().Truncate(time.Hour).Add(-48 * time.Hour)
	if _, err := repo.Increment(ctx, key, expired); err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}

	job := &river.Job[dispatcher.DeleteExpiredRateLimitsArgs]{Args: dispatcher.DeleteExpiredRateLimitsArgs{}}
	if err := w.Work(ctx, job); err != nil {
		t.Fatalf("Work()のエラー = %v", err)
	}

	// 消えた時間枠へ数え直すと1から始まる。
	revived, err := repo.Increment(ctx, key, expired)
	if err != nil {
		t.Fatalf("Increment()のエラー = %v", err)
	}
	if revived.Count != 1 {
		t.Errorf("Count = %d、期待値 = 1 (消えていることを期待)", revived.Count)
	}
}
