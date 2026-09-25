package invitation_acceptance

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cutreapp/cutre/go/internal/clientip"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/invitation_acceptance"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /i/{token} (日本語版) と GET /en/i/{token} (英語版) - 招待した人を示し、登録を始めるボタンを描画する。
//
// 使えない招待は404で、無い・使用済み・期限切れ・取り消し済みを区別せずに示す。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")

	output, ok := h.getInvitation(w, r, token)
	if !ok {
		return
	}

	h.render(w, r, http.StatusOK, page.NewPageData{
		State:     page.StateUsable,
		Token:     token,
		CSRFToken: middleware.CSRFTokenFromContext(ctx),
		Inviter:   viewmodel.InviterLabel(ctx, output.Invitation, output.Inviter),
	})
}

// getInvitation はレート制限を数えてから、トークンの招待を引く。
// 引けなかったときは応答を書き終えてfalseを返す。表示と登録の開始で同じ関門を通すために使う。
func (h *Handler) getInvitation(w http.ResponseWriter, r *http.Request, token string) (*usecase.GetInvitationOutput, bool) {
	ctx := r.Context()

	resetAt, err := h.checkRateLimit(ctx, clientip.Resolve(r, h.cfg.TrustedProxies))
	if err != nil {
		slog.ErrorContext(ctx, "招待の受け取りのレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}
	if !resetAt.IsZero() {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(resetAt).Seconds()))))
		h.render(w, r, http.StatusTooManyRequests, page.NewPageData{State: page.StateRateLimited, Token: token})
		return nil, false
	}

	output, err := h.getInvitationUC.Execute(ctx, usecase.GetInvitationInput{Token: token})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeResourceNotFound {
			h.render(w, r, http.StatusNotFound, page.NewPageData{State: page.StateUnusable, Token: token})
			return nil, false
		}
		slog.ErrorContext(ctx, "招待の取得に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, false
	}

	return output, true
}

// checkRateLimit はこのリクエストをIPアドレスの単位で数え、上限を超えていれば数え直しになる時刻を返す。
// 上限内ならゼロ値を返す。
func (h *Handler) checkRateLimit(ctx context.Context, ip string) (time.Time, error) {
	result, err := h.limiter.Check(ctx, ratelimit.CheckInput{
		Key:    ratelimit.IPKey(rateLimitAction, ip),
		Limit:  ipRateLimit,
		Window: rateLimitWindow,
	})
	if err != nil {
		return time.Time{}, err
	}
	if !result.Allowed {
		slog.WarnContext(ctx, "レート制限の上限を超えたため、招待の受け取りを受け付けません", "ip", ip, "count", result.Count)
		return result.ResetAt, nil
	}

	return time.Time{}, nil
}

// render は招待の受け取り画面を指定したステータスで描画する。
//
// トークンをURLに持つ画面のため、検索エンジンにインデックスさせない。
func (h *Handler) render(w http.ResponseWriter, r *http.Request, status int, data page.NewPageData) {
	ctx := r.Context()

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg, templates.InvitationPath(data.Token))
	meta.SetTitle(ctx, "invitation_acceptance_new_title")
	meta.Description = i18n.T(ctx, "invitation_acceptance_new_description")
	meta.NoIndex = true

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := layouts.Default(layouts.DefaultLayoutData{Meta: meta}, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "招待の受け取り画面の描画に失敗しました", "error", err)
	}
}
