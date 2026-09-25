package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestLimiter_CheckSignInTwoFactor は、同じユーザーへの試行を、IPアドレスを変えても5回までだけ受け付け、
// 6回目で数え直しの時刻を返すことを検証する。
func TestLimiter_CheckSignInTwoFactor(t *testing.T) {
	t.Parallel()

	limiter, _ := newLimiter(t)
	ctx := context.Background()
	userID := "user-" + testutil.UniqueAtname()

	for i := range 5 {
		resetAt, err := limiter.CheckSignInTwoFactor(ctx, "203.0.113."+string(rune('1'+i)), userID)
		if err != nil {
			t.Fatalf("%d回目のエラー = %v", i+1, err)
		}
		if !resetAt.IsZero() {
			t.Fatalf("%d回目の数え直しの時刻 = %v、上限内のゼロ値を期待", i+1, resetAt)
		}
	}

	resetAt, err := limiter.CheckSignInTwoFactor(ctx, "203.0.113.9", userID)
	if err != nil {
		t.Fatalf("6回目のエラー = %v", err)
	}
	if !resetAt.After(time.Now()) || resetAt.After(time.Now().Add(15*time.Minute)) {
		t.Errorf("6回目の数え直しの時刻 = %v、15分以内の未来を期待", resetAt)
	}
}

// TestWaitMinutes は、数え直しまでの時間を分に切り上げ、解除の直前でも1分と示すことを検証する。
func TestWaitMinutes(t *testing.T) {
	t.Parallel()

	for untilReset, want := range map[time.Duration]int{
		15 * time.Minute:       15,
		61 * time.Second:       2,
		500 * time.Millisecond: 1,
		0:                      1,
		-time.Second:           1,
	} {
		if got := ratelimit.WaitMinutes(untilReset); got != want {
			t.Errorf("WaitMinutes(%s) = %d、期待値 = %d", untilReset, got, want)
		}
	}
}
