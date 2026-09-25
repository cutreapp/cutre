package settings_two_factor_auth

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Delete DELETE /settings/two_factor_auth - パスワードまたは認証アプリのコードで再認証し、二要素認証を無効にする。
//
// フォームはPOSTに _method を載せて届く。無効にしたら、完了のメッセージを付けて二要素認証の画面へ戻す。
// 再認証を受け付けなかったときは、二要素認証の画面をメッセージ付きで描き直す。
// 入力はパスワードのことがあるため、入力欄には戻さない。
// 有効にしていなかったとき (別の画面で既に無効にした) は、そのまま二要素認証の画面へ戻す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の設定かを決められないまま無効にしない。
		slog.ErrorContext(ctx, "二要素認証の無効化に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	resetAt, err := h.limiter.CheckTwoFactorAuthDisable(ctx, user.ID.String())
	if err != nil {
		slog.ErrorContext(ctx, "二要素認証の無効化のレート制限の判定に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		untilReset := time.Until(resetAt)
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(untilReset.Seconds()))))
		h.renderShow(w, r, user, http.StatusTooManyRequests, rateLimitedError(ctx, untilReset))
		return
	}

	err = h.disableTwoFactorAuthUC.Execute(ctx, usecase.DisableTwoFactorAuthInput{
		UserID:     user.ID,
		Credential: r.PostFormValue("credential"),
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderShow(w, r, user, http.StatusUnprocessableEntity, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			http.Redirect(w, r, templates.SettingsTwoFactorAuthPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "二要素認証の無効化に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_two_factor_auth_disabled"))
	http.Redirect(w, r, templates.SettingsTwoFactorAuthPath, http.StatusSeeOther)
}

// rateLimitedError は試行の上限に達したことを、解除までの分数 (切り上げ) を添えたフォーム全体のエラーで返す。
func rateLimitedError(ctx context.Context, untilReset time.Duration) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, "validation_totp_rate_limited", map[string]any{"Minutes": ratelimit.WaitMinutes(untilReset)}))

	return ve
}
