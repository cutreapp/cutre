package templates_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates"
)

// TestMainNavBadgesFromContext は、載せたメインメニューの数字を読み戻し、載っていなければゼロ値を返すことを検証する。
func TestMainNavBadgesFromContext(t *testing.T) {
	t.Parallel()

	if got := templates.MainNavBadgesFromContext(context.Background()); got != (templates.MainNavBadges{}) {
		t.Errorf("載せていないとき = %+v、ゼロ値を期待", got)
	}

	ctx := templates.WithMainNavBadges(context.Background(), templates.MainNavBadges{AwaitingTradeCount: 2})
	if got := templates.MainNavBadgesFromContext(ctx); got.AwaitingTradeCount != 2 {
		t.Errorf("AwaitingTradeCount = %d、期待値 = 2", got.AwaitingTradeCount)
	}
}
