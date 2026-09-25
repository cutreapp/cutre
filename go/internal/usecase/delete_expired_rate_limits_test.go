package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestDeleteExpiredRateLimitsUsecase_Execute は、保持期間 (1日) を過ぎた時間枠のカウンターだけを消し、
// それより新しいカウンターは残すことを検証する。
func TestDeleteExpiredRateLimitsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewRateLimitRepository(db).WithTx(tx)
	uc := usecase.NewDeleteExpiredRateLimitsUsecase(ratelimit.NewLimiter(repo))

	key := "test:" + testutil.UniqueAtname()
	current := time.Now().UTC().Truncate(time.Hour)
	expired := current.Add(-48 * time.Hour)
	// 判定には使わない過ぎた時間枠でも、保持期間の内側にあれば残す。
	retained := current.Add(-2 * time.Hour)

	for _, windowStart := range []time.Time{expired, retained} {
		if _, err := repo.Increment(ctx, key, windowStart); err != nil {
			t.Fatalf("Increment()のエラー = %v", err)
		}
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	// 消えた時間枠へ数え直すと1から始まり、残った時間枠は積み上がる。
	tests := []struct {
		name        string
		windowStart time.Time
		wantCount   int32
	}{
		{name: "保持期間を過ぎた時間枠", windowStart: expired, wantCount: 1},
		{name: "保持期間内の時間枠", windowStart: retained, wantCount: 2},
	}
	for _, tt := range tests {
		rateLimit, err := repo.Increment(ctx, key, tt.windowStart)
		if err != nil {
			t.Fatalf("Increment()のエラー = %v", err)
		}
		if rateLimit.Count != tt.wantCount {
			t.Errorf("%sのCount = %d、期待値 = %d", tt.name, rateLimit.Count, tt.wantCount)
		}
	}
}

// TestDeleteExpiredRateLimitsUsecase_Execute_Error は、削除のエラーを呼び出し元へ返すことを検証する。
// 定期ジョブはこのエラーを見てRiverに再試行させる。
func TestDeleteExpiredRateLimitsUsecase_Execute_Error(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewDeleteExpiredRateLimitsUsecase(ratelimit.NewLimiter(repository.NewRateLimitRepository(db).WithTx(tx)))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := uc.Execute(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Execute()のエラー = %v、context.Canceledを期待", err)
	}
}
