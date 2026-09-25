package sign_in_two_factor

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
	page "github.com/cutreapp/cutre/go/internal/templates/pages/sign_in_two_factor"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /sign_in/two_factor (日本語版) と POST /en/sign_in/two_factor (英語版) - 認証アプリのコードを照合し、
// 合っていればセッションを発行する。
//
// 照合する相手は、パスワードを確かめたときに発行した署名付きのCookieから読む。フォームの値からは読まない。
// 受け付けなかった送信はフォームをメッセージ付きで再描画し、入力したコードを戻す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PostFormValue("code")
	// ログイン後の行き先はフォームから届くため、リダイレクト先に使う前に検証する。
	returnTo := middleware.SanitizeReturnTo(r.PostFormValue(templates.ReturnToParam))

	userID, ok := h.continuationMgr.TwoFactorPendingUserID(r)
	if !ok {
		redirectToSignIn(w, r, returnTo)
		return
	}

	rerender := func(status int, formErrors *model.ValidationError) {
		h.render(w, r, status, page.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Code:       code,
			FormErrors: formErrors,
			ReturnTo:   returnTo,
		})
	}

	ip := clientip.Resolve(r, h.cfg.TrustedProxies)

	resetAt, err := h.limiter.CheckSignInTwoFactor(ctx, ip, userID.String())
	if err != nil {
		slog.ErrorContext(ctx, "認証アプリのコードの照合のレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		untilReset := time.Until(resetAt)
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(untilReset.Seconds()))))
		rerender(http.StatusTooManyRequests, rateLimitedError(ctx, untilReset))
		return
	}

	signInOutput, err := h.createSignInTwoFactorUC.Execute(ctx, usecase.CreateSignInTwoFactorInput{
		UserID: userID,
		Code:   code,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			rerender(http.StatusUnprocessableEntity, ve)
			return
		}
		// パスワードを確かめた後に退会した・二要素認証を無効にしたときは、パスワードの確認からやり直させる。
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			h.continuationMgr.DeleteTwoFactorPendingUserID(w)
			redirectToSignIn(w, r, returnTo)
			return
		}
		slog.ErrorContext(ctx, "認証アプリのコードの照合に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user := signInOutput.User

	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    user.ID,
		IPAddress: ip,
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// コードは合ったが、セッションを作れなかった。黙って未ログインのままにせず500にする。
		slog.ErrorContext(ctx, "セッションの作成に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)
	h.continuationMgr.DeleteTwoFactorPendingUserID(w)

	// 行き先のページは users.locale の言語で表示されるため、メッセージもその言語にする (ログインと同じ)。
	h.flashMgr.SetSuccess(w, i18n.T(i18n.SetLocale(ctx, string(user.Locale)), "flash_sign_in_success"))

	destination := templates.HomePath
	if returnTo != "" {
		destination = returnTo
	}
	// 行き先はフォームから届く値だが、SanitizeReturnTo が同一オリジンの相対パスだけに絞っている。
	// gosecのG710 (オープンリダイレクト) は検証を追えずに警告するため、ここでは誤検知。
	//nolint:gosec // G710
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

// rateLimitedError は、試行の上限に達したことと、次に試せるまでの分数 (切り上げ) をフォーム全体のエラーで返す。
func rateLimitedError(ctx context.Context, untilReset time.Duration) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, "validation_totp_rate_limited", map[string]any{"Minutes": ratelimit.WaitMinutes(untilReset)}))

	return ve
}
