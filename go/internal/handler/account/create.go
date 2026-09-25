package account

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/clientip"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/account"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /account と POST /en/account - アカウントを作成し、そのままログインさせてホームへ送る。
//
// 受け付けなかった送信はフォームをメッセージ付きで再描画する。アットネームは戻し、パスワードは戻さない。
// Bot対策とレート制限は掛けない。確認済みのCookieは、Bot対策とレート制限を通った登録の開始と
// 確認コードの照合を経なければ発行されないため。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	atname := r.PostFormValue("atname")

	invitationID, confirmation, ok := h.usableContinuation(w, r)
	if !ok {
		return
	}

	accountOutput, err := h.createAccountUC.Execute(ctx, usecase.CreateAccountInput{
		InvitationID:        invitationID,
		EmailConfirmationID: confirmation.ID,
		Atname:              atname,
		Password:            r.PostFormValue("password"),
		Locale:              model.Locale(i18n.GetLocale(ctx)),
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.render(w, r, http.StatusUnprocessableEntity, page.NewPageData{
				CSRFToken:  middleware.CSRFTokenFromContext(ctx),
				Email:      confirmation.Email,
				Atname:     atname,
				FormErrors: ve,
			})
			return
		}
		if ae := model.AsAppError(err); ae != nil {
			switch ae.Code {
			case model.AppErrCodeForbidden:
				h.renderInvitationUnusable(w, r)
				return
			case model.AppErrCodeResourceNotFound:
				h.continuationMgr.DeleteConfirmedEmailConfirmationID(w)
				http.Redirect(w, r, templates.LocalePath(ctx, templates.SignUpPath), http.StatusSeeOther)
				return
			}
		}
		slog.ErrorContext(ctx, "アカウントの作成に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// アカウントは作り終えたため、成否によらず登録の途中の状態は消す。
	// 残すと、使い終えた確認で作成をやり直すことになる。
	h.continuationMgr.DeleteInvitationID(w)
	h.continuationMgr.DeleteConfirmedEmailConfirmationID(w)

	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    accountOutput.User.ID,
		IPAddress: clientip.Resolve(r, h.cfg.TrustedProxies),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// アカウントはできているため、利用者はログイン画面からログインできる。
		slog.ErrorContext(ctx, "アカウントの作成後のセッションの作成に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_account_created"))

	http.Redirect(w, r, templates.HomePath, http.StatusSeeOther)
}
