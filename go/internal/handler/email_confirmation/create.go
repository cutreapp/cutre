package email_confirmation

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
	page "github.com/cutreapp/cutre/go/internal/templates/pages/email_confirmation"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /email_confirmation と POST /en/email_confirmation - 確認コードを照合し、アカウントの作成へ送る。
//
// 確認を済ませたら、確認コードを入力する前のCookieを、アカウントの作成まで運ぶ確認済みのCookieへ置き換える。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.PostFormValue("code")

	confirmationID, invitationID, ok := h.usableContinuation(w, r)
	if !ok {
		return
	}

	rerender := func(status int, formErrors *model.ValidationError) {
		h.render(w, r, status, page.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Code:       code,
			FormErrors: formErrors,
		})
	}

	ip := clientip.Resolve(r, h.cfg.TrustedProxies)
	resetAt, err := h.checkRateLimit(ctx, ip, []ratelimit.CheckInput{
		{Key: ratelimit.IPKey(verifyRateLimitAction, ip), Limit: verifyIPRateLimit, Window: verifyRateLimitWindow},
	})
	if err != nil {
		slog.ErrorContext(ctx, "確認コードの照合のレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		setRetryAfter(w, resetAt)
		rerender(http.StatusTooManyRequests, globalError(ctx, "validation_rate_limited"))
		return
	}

	output, err := h.verifyEmailConfirmationUC.Execute(ctx, usecase.VerifyEmailConfirmationInput{
		ID:   confirmationID,
		Code: code,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			rerender(http.StatusUnprocessableEntity, ve)
			return
		}
		slog.ErrorContext(ctx, "確認コードの照合に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.continuationMgr.DeleteEmailConfirmationID(w)
	h.continuationMgr.SetInvitationID(w, invitationID)
	h.continuationMgr.SetConfirmedEmailConfirmationID(w, output.EmailConfirmation.ID)

	http.Redirect(w, r, templates.LocalePath(ctx, templates.AccountPath), http.StatusSeeOther)
}

// checkRateLimit はこのリクエストを渡された単位ごとに数え、どれかが上限を超えていれば全ての制限が解除される時刻を返す。
// どれも上限内ならゼロ値を返す。
//
// 1つが超えていても残りを数える理由は、ログインと同じ (攻撃の規模を後から把握できるようにする)。
func (h *Handler) checkRateLimit(ctx context.Context, ip string, checks []ratelimit.CheckInput) (time.Time, error) {
	var resetAt time.Time
	for _, check := range checks {
		result, err := h.limiter.Check(ctx, check)
		if err != nil {
			return time.Time{}, err
		}
		if !result.Allowed {
			// キーはメールアドレスを含みうるため、ログにはIPアドレスだけを出す。
			slog.WarnContext(ctx, "レート制限の上限を超えたため、確認コードの操作を受け付けません", "ip", ip, "count", result.Count)
			if result.ResetAt.After(resetAt) {
				resetAt = result.ResetAt
			}
		}
	}

	return resetAt, nil
}

// setRetryAfter は数え直しになるまでの秒数をRetry-Afterに入れる。
func setRetryAfter(w http.ResponseWriter, resetAt time.Time) {
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(resetAt).Seconds()))))
}

// globalError はフォーム全体に関わる1つのエラーを持つ ValidationError を返す。
func globalError(ctx context.Context, key string) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, key))

	return ve
}
