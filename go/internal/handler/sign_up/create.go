package sign_up

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cutreapp/cutre/go/internal/clientip"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/templates"
	signuppage "github.com/cutreapp/cutre/go/internal/templates/pages/sign_up"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /sign_up (日本語版) と POST /en/sign_up (英語版) - 確認コードを送り、コードの入力画面へ送る。
//
// 登録済みのメールアドレスでも、応答は未登録のときと同じにする (UseCaseが送るメールだけを変える)。
// Bot対策とレート制限は、フォームの内容によらないリクエスト単位の関門としてUseCaseより前に置く。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := r.PostFormValue("email")

	rerender := func(status int, formErrors *model.ValidationError) {
		h.render(w, r, status, signuppage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
		})
	}

	invitationID, hasInvitation, ok := h.usableInvitationID(w, r)
	if !ok {
		return
	}
	if !hasInvitation {
		h.render(w, r, http.StatusForbidden, signuppage.NewPageData{InvitationRequired: true})
		return
	}

	passed, err := h.turnstile.Verify(ctx, r.PostFormValue("cf-turnstile-response"))
	if err != nil || !passed {
		// 単なる非通過 (トークンが空) はBotの想定内の拒否のため、errorがあるときだけ添える。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstileの確認を通過しなかったため、登録を受け付けません", attrs...)
		rerender(http.StatusUnprocessableEntity, globalError(ctx, "validation_turnstile_failed"))
		return
	}

	resetAt, err := h.checkRateLimit(ctx, clientip.Resolve(r, h.cfg.TrustedProxies), email)
	if err != nil {
		slog.ErrorContext(ctx, "登録のレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(resetAt).Seconds()))))
		rerender(http.StatusTooManyRequests, globalError(ctx, "validation_rate_limited"))
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
			h.render(w, r, http.StatusForbidden, signuppage.NewPageData{InvitationRequired: true})
			return
		}
		if ve := model.AsValidationError(err); ve != nil {
			rerender(http.StatusUnprocessableEntity, ve)
			return
		}
		slog.ErrorContext(ctx, "登録の開始に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.continuationMgr.SetInvitationID(w, invitationID)
	h.continuationMgr.SetEmailConfirmationID(w, output.EmailConfirmation.ID)

	http.Redirect(w, r, templates.LocalePath(ctx, templates.EmailConfirmationPath), http.StatusSeeOther)
}

// checkRateLimit はこの送信をIPアドレスとメールアドレスの両方の単位で数え、どちらかが上限を超えていれば
// 数え直しになる時刻を返す。どちらも上限内ならゼロ値を返す。
//
// 片方が超えていても両方を数える理由は、ログインと同じ (攻撃の規模を後から把握できるようにする)。
// メールアドレスが空の送信は宛先を持たないため、IPアドレスの単位だけで数える。
func (h *Handler) checkRateLimit(ctx context.Context, ip, email string) (time.Time, error) {
	checks := []ratelimit.CheckInput{
		{Key: ratelimit.IPKey(rateLimitAction, ip), Limit: ipRateLimit, Window: rateLimitWindow},
	}
	if email != "" {
		checks = append(checks, ratelimit.CheckInput{Key: ratelimit.EmailKey(rateLimitAction, email), Limit: emailRateLimit, Window: rateLimitWindow})
	}

	var resetAt time.Time
	for _, check := range checks {
		result, err := h.limiter.Check(ctx, check)
		if err != nil {
			return time.Time{}, err
		}
		if !result.Allowed {
			// メールアドレスはログに出さない。
			slog.WarnContext(ctx, "レート制限の上限を超えたため、登録を受け付けません", "ip", ip, "count", result.Count)
			if result.ResetAt.After(resetAt) {
				resetAt = result.ResetAt
			}
		}
	}

	return resetAt, nil
}

// globalError はフォーム全体に関わる1つのエラーを持つ ValidationError を返す。
func globalError(ctx context.Context, key string) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, key))

	return ve
}
