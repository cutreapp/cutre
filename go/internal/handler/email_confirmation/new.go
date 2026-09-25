package email_confirmation

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /email_confirmation と GET /en/email_confirmation - 確認コードの入力画面を描画する。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.usableContinuation(w, r); !ok {
		return
	}

	h.render(w, r, http.StatusOK, page.NewPageData{
		CSRFToken: middleware.CSRFTokenFromContext(r.Context()),
	})
}

// usableContinuation はCookieが運ぶ確認と招待のIDを返す。
// どちらかが無い、または招待が登録に使えなくなっているときは、登録の画面へ送り、第3の戻り値をfalseにする。
// 招待を引けないときは500を返し、同じくfalseにする。
//
// 招待は、確認コードを送ってからの間にも取り消し・使用済みにされうるため、各手順で改めて確かめる。
func (h *Handler) usableContinuation(w http.ResponseWriter, r *http.Request) (model.EmailConfirmationID, model.InvitationID, bool) {
	ctx := r.Context()
	signUpPath := templates.LocalePath(ctx, templates.SignUpPath)

	confirmationID, ok := h.continuationMgr.EmailConfirmationID(r)
	if !ok {
		http.Redirect(w, r, signUpPath, http.StatusSeeOther)
		return model.EmailConfirmationID{}, model.InvitationID{}, false
	}

	invitationID, ok := h.continuationMgr.InvitationID(r)
	if !ok {
		http.Redirect(w, r, signUpPath, http.StatusSeeOther)
		return model.EmailConfirmationID{}, model.InvitationID{}, false
	}
	if _, err := h.getInvitationByIDUC.Execute(ctx, invitationID); err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.continuationMgr.DeleteInvitationID(w)
			http.Redirect(w, r, signUpPath, http.StatusSeeOther)
			return model.EmailConfirmationID{}, model.InvitationID{}, false
		}
		slog.ErrorContext(ctx, "確認コードの画面で招待の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return model.EmailConfirmationID{}, model.InvitationID{}, false
	}

	return confirmationID, invitationID, true
}

// render は確認コードの入力画面を指定したステータスで描画する。
// 表示 (200) と、受け付けなかった照合・再送の再描画 (422 / 429) で共有する。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.EmailConfirmationPath)
	meta.SetTitle(ctx, "email_confirmation_new_title")
	meta.Description = i18n.T(ctx, "email_confirmation_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "確認コードの画面の描画に失敗しました", "error", err)
	}
}
