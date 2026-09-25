package sign_in

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	signinpage "github.com/cutreapp/cutre/go/internal/templates/pages/sign_in"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /sign_in (日本語版) と GET /en/sign_in (英語版) - ログイン画面を描画する。
//
// ログインを求めるページから送られてきた訪問者は、行こうとしていた先を return_to に持って届く。
// 行き先として使える値だけをフォームへ載せるため、ここで検証してから渡す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.render(w, r, http.StatusOK, signinpage.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		ReturnTo:  middleware.SanitizeReturnTo(r.URL.Query().Get(templates.ReturnToParam)),
	})
}

// render はログイン画面を指定したステータスで描画する。
// 表示 (200) と、送信を受け付けなかったときの再描画 (422 / 429) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data signinpage.NewPageData) {
	ctx := r.Context()

	// サイトキーはリクエストによらず設定で決まるため、呼び出し側ごとではなくここで入れる。
	data.TurnstileSiteKey = h.cfg.TurnstileSiteKey

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.SignInPath)
	meta.AddTurnstilePreconnect(data.TurnstileSiteKey)
	meta.SetTitle(ctx, "sign_in_new_title")
	meta.Description = i18n.T(ctx, "sign_in_new_description")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, signinpage.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "ログイン画面の描画に失敗しました", "error", err)
	}
}
