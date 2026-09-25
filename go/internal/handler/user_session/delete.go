package user_session

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Delete DELETE /user_session - セッションを削除してCookieを消し、トップページへ送る。
//
// セッションの行をCookieより先に消し、Cookieが残っていても使えないようにする。
// ログインしていないリクエストでも失敗させず、同じ行き先へ送る。ログアウトは何度行っても同じ結果に落ち着くべき操作のため。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 行き先とメッセージの言語は、ログアウトする利用者の言語にする。
	// ログイン後のページは users.locale で表示していたため、URLの言語 (このパスは常に日本語) にすると
	// 英語で使っていた利用者が日本語のトップページへ送られる。
	locale := i18n.GetLocale(ctx)
	if user := middleware.UserFromContext(ctx); user != nil {
		locale = string(user.Locale)
	}

	if err := h.deleteSessionUC.Execute(ctx, usecase.DeleteSessionInput{Token: h.sessionMgr.SessionToken(r)}); err != nil {
		slog.ErrorContext(ctx, "ログアウトに失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.DeleteSessionCookie(w)
	h.flashMgr.SetSuccess(w, i18n.T(i18n.SetLocale(ctx, locale), "flash_sign_out_success"))
	http.Redirect(w, r, i18n.LocalePath(locale, "/"), http.StatusSeeOther)
}
