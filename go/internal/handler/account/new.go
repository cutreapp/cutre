package account

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/account"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /account と GET /en/account - アットネームとパスワードの入力画面を描画する。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	_, confirmation, ok := h.usableContinuation(w, r)
	if !ok {
		return
	}

	h.render(w, r, http.StatusOK, page.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(r.Context()),
		Email:     confirmation.Email,
	})
}

// usableContinuation はCookieが運ぶ招待のIDと、確認を済ませた確認を返す。
// 手順を続けられないときは応答を書き終え、第3の戻り値をfalseにする。
//
// Cookieが無い、または確認が引けないときは、登録の画面からやり直させる。
// 招待が使えなくなっているときは、登録の画面へ戻しても招待制の案内しか出ないため、ここで理由を示す。
func (h *Handler) usableContinuation(w http.ResponseWriter, r *http.Request) (model.InvitationID, *model.EmailConfirmation, bool) {
	ctx := r.Context()
	signUpPath := templates.LocalePath(ctx, templates.SignUpPath)

	invitationID, hasInvitation := h.continuationMgr.InvitationID(r)
	confirmationID, hasConfirmation := h.continuationMgr.ConfirmedEmailConfirmationID(r)
	if !hasInvitation || !hasConfirmation {
		http.Redirect(w, r, signUpPath, http.StatusSeeOther)
		return model.InvitationID{}, nil, false
	}

	if _, err := h.getInvitationByIDUC.Execute(ctx, invitationID); err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.renderInvitationUnusable(w, r)
			return model.InvitationID{}, nil, false
		}
		slog.ErrorContext(ctx, "アカウントの作成の画面で招待の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return model.InvitationID{}, nil, false
	}

	output, err := h.getConfirmedEmailConfirmationUC.Execute(ctx, confirmationID)
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.continuationMgr.DeleteConfirmedEmailConfirmationID(w)
			http.Redirect(w, r, signUpPath, http.StatusSeeOther)
			return model.InvitationID{}, nil, false
		}
		slog.ErrorContext(ctx, "アカウントの作成の画面で確認済みのメールアドレスの確認の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return model.InvitationID{}, nil, false
	}

	return invitationID, output.EmailConfirmation, true
}

// renderInvitationUnusable は、招待が使えなくなったため登録を完了できないことを403で示す。
// 使えない招待と確認のCookieは消す。残しても、どの手順でも先へ進めないため。
func (h *Handler) renderInvitationUnusable(w http.ResponseWriter, r *http.Request) {
	h.continuationMgr.DeleteInvitationID(w)
	h.continuationMgr.DeleteConfirmedEmailConfirmationID(w)
	h.render(w, r, http.StatusForbidden, page.NewPageData{InvitationUnusable: true})
}

// render はアカウントの作成画面を指定したステータスで描画する。
// 表示 (200)、使えない招待の案内 (403)、受け付けなかった送信の再描画 (422) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.AccountPath)
	meta.SetTitle(ctx, "account_new_title")
	meta.Description = i18n.T(ctx, "account_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "アカウントの作成画面の描画に失敗しました", "error", err)
	}
}
