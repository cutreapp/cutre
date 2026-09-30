package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestLimiter_CheckTradeMessage は、同じユーザーの交換のメッセージを10分に30通までだけ受け付け、31通目で数え直しの時刻を返し、
// ほかのユーザーの回数とは分けて数えることを検証する。
func TestLimiter_CheckTradeMessage(t *testing.T) {
	t.Parallel()

	limiter, _ := newLimiter(t)
	ctx := context.Background()
	userID := "user-" + testutil.UniqueAtname()

	for i := range 30 {
		resetAt, err := limiter.CheckTradeMessage(ctx, userID)
		if err != nil {
			t.Fatalf("%d通目のエラー = %v", i+1, err)
		}
		if !resetAt.IsZero() {
			t.Fatalf("%d通目の数え直しの時刻 = %v、上限内のゼロ値を期待", i+1, resetAt)
		}
	}

	resetAt, err := limiter.CheckTradeMessage(ctx, userID)
	if err != nil {
		t.Fatalf("31通目のエラー = %v", err)
	}
	if !resetAt.After(time.Now()) || resetAt.After(time.Now().Add(10*time.Minute)) {
		t.Errorf("31通目の数え直しの時刻 = %v、10分以内の未来を期待", resetAt)
	}

	otherResetAt, err := limiter.CheckTradeMessage(ctx, "user-"+testutil.UniqueAtname())
	if err != nil || !otherResetAt.IsZero() {
		t.Errorf("ほかのユーザーの1通目 = (%v, %v)、(ゼロ値, nil) を期待", otherResetAt, err)
	}
}
