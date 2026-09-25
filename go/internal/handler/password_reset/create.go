package password_reset

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
	page "github.com/cutreapp/cutre/go/internal/templates/pages/password_reset"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /password_reset (日本語版) と POST /en/password_reset (英語版) - 申請を受け付け、受け付けた後の画面へ送る。
//
// 登録済みかどうかによらず同じ画面へ送り、登録の有無を明かさない。
// Bot対策とレート制限は、フォームの内容によらないリクエスト単位の関門としてUseCaseより前に置く。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := r.PostFormValue("email")

	rerender := func(status int, formErrors *model.ValidationError) {
		h.render(w, r, status, page.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
		})
	}

	passed, err := h.turnstile.Verify(ctx, r.PostFormValue("cf-turnstile-response"))
	if err != nil || !passed {
		// 単なる非通過 (トークンが空) はBotの想定内の拒否のため、errorがあるときだけ添える。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstileの確認を通過しなかったため、パスワードリセットの申請を受け付けません", attrs...)
		rerender(http.StatusUnprocessableEntity, globalError(ctx, "validation_turnstile_failed"))
		return
	}

	resetAt, err := h.checkRateLimit(ctx, clientip.Resolve(r, h.cfg.TrustedProxies), email)
	if err != nil {
		slog.ErrorContext(ctx, "パスワードリセットの申請のレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(resetAt).Seconds()))))
		rerender(http.StatusTooManyRequests, globalError(ctx, "validation_rate_limited"))
		return
	}

	if err := h.createPasswordResetUC.Execute(ctx, usecase.CreatePasswordResetInput{
		Email:  email,
		Locale: model.Locale(i18n.GetLocale(ctx)),
	}); err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			rerender(http.StatusUnprocessableEntity, ve)
			return
		}
		// 500を返さず、受け付けた後の画面へ送る。ジョブの投入は登録済みのアドレスでしか行わないため、
		// その失敗だけを500にすると、応答の違いから登録の有無が分かってしまう。
		slog.ErrorContext(ctx, "パスワードリセットの申請の処理に失敗しました", "error", err)
	}

	http.Redirect(w, r, templates.LocalePath(ctx, templates.PasswordResetSentPath), http.StatusSeeOther)
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
			slog.WarnContext(ctx, "レート制限の上限を超えたため、パスワードリセットの申請を受け付けません", "ip", ip, "count", result.Count)
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
