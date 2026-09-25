package email_confirmation

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/clientip"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/templates"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Update PATCH /email_confirmation と PATCH /en/email_confirmation - 同じメールアドレスへ新しい確認コードを送り直す。
//
// 送り先は入力し直させず、Cookieが運ぶ確認のメールアドレスを使う。
// 登録の開始と同じUseCaseで新しい確認を作るため、登録済みのアドレスには確認コードではなくログインの案内が届く。
// Bot対策は掛けない。確認のCookieは、Bot対策を通った登録の開始でしか発行されないため。
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	signUpPath := templates.LocalePath(ctx, templates.SignUpPath)

	confirmationID, invitationID, ok := h.usableContinuation(w, r)
	if !ok {
		return
	}

	current, err := h.getEmailConfirmationUC.Execute(ctx, confirmationID)
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.continuationMgr.DeleteEmailConfirmationID(w)
			http.Redirect(w, r, signUpPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "再送するメールアドレスの確認の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	email := current.EmailConfirmation.Email

	ip := clientip.Resolve(r, h.cfg.TrustedProxies)
	resetAt, err := h.checkRateLimit(ctx, ip, []ratelimit.CheckInput{
		{Key: ratelimit.IPKey(resendRateLimitAction, ip), Limit: resendIPRateLimit, Window: resendRateLimitWindow},
		{Key: ratelimit.EmailKey(resendRateLimitAction, email), Limit: resendEmailRateLimit, Window: resendRateLimitWindow},
	})
	if err != nil {
		slog.ErrorContext(ctx, "確認コードの再送のレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		setRetryAfter(w, resetAt)
		h.render(w, r, http.StatusTooManyRequests, page.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			FormErrors: globalError(ctx, "validation_rate_limited"),
		})
		return
	}

	output, err := h.createSignUpUC.Execute(ctx, usecase.CreateSignUpInput{
		InvitationID: invitationID,
		Email:        email,
		Locale:       model.Locale(i18n.GetLocale(ctx)),
	})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.continuationMgr.DeleteInvitationID(w)
			http.Redirect(w, r, signUpPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "確認コードの再送に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.continuationMgr.SetInvitationID(w, invitationID)
	h.continuationMgr.SetEmailConfirmationID(w, output.EmailConfirmation.ID)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_email_confirmation_resent"))

	http.Redirect(w, r, templates.LocalePath(ctx, templates.EmailConfirmationPath), http.StatusSeeOther)
}
