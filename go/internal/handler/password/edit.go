package password

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/password"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Edit GET /password と GET /en/password - 新しいパスワードの設定画面を描画する。
//
// メールのリンク (?token=) から開いたときは、トークンを確かめてCookieへ移し、トークンを持たない同じ画面へ303で送る。
// トークンをURLに残したまま画面を描画すると、ブラウザの履歴や、画面から送る要求に残るため。
// 使えないトークンと、Cookieが無い・使えないときは、無い・期限切れ・使用済みを区別せずに404で示す。
func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query()
	if _, exists := query["token"]; exists {
		h.acceptToken(w, r, query.Get("token"))
		return
	}

	output, ok := h.usableToken(w, r)
	if !ok {
		return
	}

	h.render(w, r, http.StatusOK, page.EditPageData{
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		Email:     output.User.Email,
	})
}

// acceptToken はリンクのトークンを確かめ、使えるときはそのIDをCookieに書き込んで、トークンを持たない画面へ送る。
func (h *Handler) acceptToken(w http.ResponseWriter, r *http.Request, token string) {
	ctx := r.Context()

	output, err := h.getPasswordResetTokenUC.Execute(ctx, usecase.GetPasswordResetTokenInput{Token: token})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.renderUnusable(w, r)
			return
		}
		slog.ErrorContext(ctx, "パスワードリセットのトークンの取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.continuationMgr.SetPasswordResetTokenID(w, output.PasswordResetToken.ID)
	http.Redirect(w, r, templates.LocalePath(ctx, templates.PasswordPath), http.StatusSeeOther)
}

// usableToken はCookieが運ぶIDのトークンと、その持ち主を返す。
// 使えないときは応答を書き終え、第2の戻り値をfalseにする。
func (h *Handler) usableToken(w http.ResponseWriter, r *http.Request) (*usecase.GetPasswordResetTokenByIDOutput, bool) {
	ctx := r.Context()

	id, ok := h.continuationMgr.PasswordResetTokenID(r)
	if !ok {
		h.renderUnusable(w, r)
		return nil, false
	}

	output, err := h.getPasswordResetTokenByIDUC.Execute(ctx, id)
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.renderUnusable(w, r)
			return nil, false
		}
		slog.ErrorContext(ctx, "新しいパスワードの設定の画面でトークンの取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}

	return output, true
}

// renderUnusable は、リンクが使えないため新しいパスワードを設定できないことを404で示す。
// 使えないトークンのCookieは消す。残しても先へ進めないため。
func (h *Handler) renderUnusable(w http.ResponseWriter, r *http.Request) {
	h.continuationMgr.DeletePasswordResetTokenID(w)
	h.render(w, r, http.StatusNotFound, page.EditPageData{Unusable: true})
}

// render は新しいパスワードの設定画面を指定したステータスで描画する。
// 表示 (200)、使えないリンクの案内 (404)、受け付けなかった送信の再描画 (422) で共有する。
//
// トークンで開くリセットの途中の画面のため、検索エンジンにインデックスさせない。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.EditPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.PasswordPath)
	meta.SetTitle(ctx, "password_edit_title")
	meta.Description = i18n.T(ctx, "password_edit_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.Edit(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "新しいパスワードの設定画面の描画に失敗しました", "error", err)
	}
}
