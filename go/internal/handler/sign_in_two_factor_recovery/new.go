package sign_in_two_factor_recovery

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/sign_in_two_factor_recovery"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /sign_in/two_factor/recovery (日本語版) と GET /en/sign_in/two_factor/recovery (英語版) - リカバリーコードの入力画面を描画する。
//
// パスワードを確かめたユーザーのCookieが無い・期限が切れたときは、コードを照合する相手がいないためログイン画面へ送る。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	returnTo := middleware.SanitizeReturnTo(r.URL.Query().Get(templates.ReturnToParam))

	if _, ok := h.continuationMgr.TwoFactorPendingUserID(r); !ok {
		redirectToSignIn(w, r, returnTo)
		return
	}

	h.render(w, r, http.StatusOK, page.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		ReturnTo:  returnTo,
	})
}

// redirectToSignIn は、表示中の言語版のログイン画面へ戻り先を引き継いで送る。
func redirectToSignIn(w http.ResponseWriter, r *http.Request, returnTo string) {
	destination := templates.WithReturnTo(templates.LocalePath(r.Context(), templates.SignInPath), returnTo)
	// 戻り先は SanitizeReturnTo で同一オリジンの相対パスに絞ってからクエリへ載せており、行き先はログイン画面に固定されている。
	//nolint:gosec // G710
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

// render はリカバリーコードの入力画面を指定したステータスで描画する。
// 表示 (200) と、送信を受け付けなかったときの再描画 (422 / 429) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.SignInTwoFactorRecoveryPath)
	meta.SetTitle(ctx, "sign_in_two_factor_recovery_new_title")
	meta.Description = i18n.T(ctx, "sign_in_two_factor_recovery_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "リカバリーコードの入力画面の描画に失敗しました", "error", err)
	}
}
