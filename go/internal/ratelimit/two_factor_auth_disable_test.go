package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestLimiter_CheckTwoFactorAuthDisable は、同じユーザーの再認証を5回までだけ受け付け、6回目で数え直しの時刻を返し、
// ほかのユーザーの回数とは分けて数えることを検証する。
func TestLimiter_CheckTwoFactorAuthDisable(t *testing.T) {
	t.Parallel()

	limiter, _ := newLimiter(t)
	ctx := context.Background()
	userID := "user-" + testutil.UniqueAtname()

	for i := range 5 {
		resetAt, err := limiter.CheckTwoFactorAuthDisable(ctx, userID)
		if err != nil {
			t.Fatalf("%d回目のエラー = %v", i+1, err)
		}
		if !resetAt.IsZero() {
			t.Fatalf("%d回目の数え直しの時刻 = %v、上限内のゼロ値を期待", i+1, resetAt)
		}
	}

	resetAt, err := limiter.CheckTwoFactorAuthDisable(ctx, userID)
	if err != nil {
		t.Fatalf("6回目のエラー = %v", err)
	}
	if !resetAt.After(time.Now()) || resetAt.After(time.Now().Add(15*time.Minute)) {
		t.Errorf("6回目の数え直しの時刻 = %v、15分以内の未来を期待", resetAt)
	}

	otherResetAt, err := limiter.CheckTwoFactorAuthDisable(ctx, "user-"+testutil.UniqueAtname())
	if err != nil || !otherResetAt.IsZero() {
		t.Errorf("ほかのユーザーの1回目 = (%v, %v)、(ゼロ値, nil) を期待", otherResetAt, err)
	}
}
