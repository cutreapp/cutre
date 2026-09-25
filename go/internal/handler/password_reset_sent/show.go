package password_reset_sent

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/password_reset_sent"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /password_reset/sent (日本語版) と GET /en/password_reset/sent (英語版) - 申請を受け付けた後の画面を描画する。
//
// 申請の手順の途中の画面のため、検索エンジンにはインデックスさせない。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.PasswordResetSentPath)
	meta.SetTitle(ctx, "password_reset_sent_show_title")
	meta.Description = i18n.T(ctx, "password_reset_sent_show_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.Show()).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "パスワードリセットの申請を受け付けた後の画面の描画に失敗しました", "error", err)
	}
}
