package trade_message

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cutreapp/cutre/go/internal/templates"
)

// TestRenderMessagePage は、描画が成功してから既読を記録し、記録に失敗したときは元の未読バッジで描き直すことを検証する。
// ミドルウェアが3件と数え、このページで2件を読む場面で確かめる。
func TestRenderMessagePage(t *testing.T) {
	t.Parallel()

	ctx := templates.WithMainNavBadges(context.Background(), templates.MainNavBadges{UnreadMessageCount: 3})
	// renderBadge は、描画に渡されたcontextのナビの未読の数を、描画の結果として返す。
	renderBadge := func(renderCtx context.Context) ([]byte, error) {
		return fmt.Appendf(nil, "badge=%d", templates.MainNavBadgesFromContext(renderCtx).UnreadMessageCount), nil
	}

	t.Run("描画に失敗したとき", func(t *testing.T) {
		t.Parallel()

		marked := false
		_, err := renderMessagePage(ctx, true, 2, func(context.Context) ([]byte, error) {
			return nil, errors.New("描画エラー")
		}, func() error {
			marked = true
			return nil
		})
		if err == nil || marked {
			t.Errorf("renderMessagePage() = (err: %v, 記録: %v)、エラーを返して既読を記録しないことを期待", err, marked)
		}
	})

	t.Run("既読の記録に失敗したとき", func(t *testing.T) {
		t.Parallel()

		html, err := renderMessagePage(ctx, true, 2, renderBadge, func() error {
			return errors.New("記録エラー")
		})
		if err != nil || string(html) != "badge=3" {
			t.Errorf("renderMessagePage() = (%q, %v)、未読のバッジを3件に戻して描き直すことを期待", html, err)
		}
	})

	t.Run("描画と記録に成功したとき", func(t *testing.T) {
		t.Parallel()

		marked := false
		html, err := renderMessagePage(ctx, true, 2, renderBadge, func() error {
			marked = true
			return nil
		})
		if err != nil || !marked || string(html) != "badge=1" {
			t.Errorf("renderMessagePage() = (%q, %v, 記録: %v)、既読を記録して未読のバッジを1件にすることを期待", html, err, marked)
		}
	})

	t.Run("既読を記録しないとき", func(t *testing.T) {
		t.Parallel()

		html, err := renderMessagePage(ctx, false, 2, renderBadge, func() error {
			t.Error("既読を記録しないときに記録を呼んだ")
			return nil
		})
		if err != nil || string(html) != "badge=3" {
			t.Errorf("renderMessagePage() = (%q, %v)、未読のバッジを3件のまま描くことを期待", html, err)
		}
	})
}
