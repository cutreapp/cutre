package email_confirmation

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestCheckRateLimit_UsesLatestResetAt は、複数の制限を超えたときに最後の検査結果ではなく遅い解除時刻を返すことを検証する。
func TestCheckRateLimit_UsesLatestResetAt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	limiter := ratelimit.NewLimiter(repository.NewRateLimitRepository(testutil.GetTestDB()))
	handler := &Handler{limiter: limiter}
	key := uuid.NewString()
	checks := []ratelimit.CheckInput{
		{Key: key + ":long", Limit: 1, Window: 365 * 24 * time.Hour},
		{Key: key + ":short", Limit: 1, Window: time.Second},
	}
	var latest time.Time
	for _, check := range checks {
		result, err := limiter.Check(ctx, check)
		if err != nil {
			t.Fatalf("事前の試行の記録に失敗: %v", err)
		}
		if result.ResetAt.After(latest) {
			latest = result.ResetAt
		}
	}

	resetAt, err := handler.checkRateLimit(ctx, "192.0.2.99", checks)
	if err != nil {
		t.Fatalf("checkRateLimit()のエラー = %v", err)
	}
	if !resetAt.Equal(latest) {
		t.Errorf("解除時刻 = %s、最も遅い %s を期待", resetAt, latest)
	}
}
