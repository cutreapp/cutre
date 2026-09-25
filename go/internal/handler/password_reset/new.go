package password_reset

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/password_reset"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /password_reset (日本語版) と GET /en/password_reset (英語版) - パスワードリセットの申請の画面を描画する。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, page.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(r.Context()),
	})
}

// render はパスワードリセットの申請の画面を指定したステータスで描画する。
// 表示 (200) と、受け付けなかった送信の再描画 (422 / 429) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.NewPageData) {
	ctx := r.Context()

	// サイトキーはリクエストによらず設定で決まるため、呼び出し側ごとではなくここで入れる。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.PasswordResetPath)
	meta.AddTurnstilePreconnect(data.TurnstileSiteKey)
	meta.SetTitle(ctx, "password_reset_new_title")
	meta.Description = i18n.T(ctx, "password_reset_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "パスワードリセットの申請の画面の描画に失敗しました", "error", err)
	}
}
