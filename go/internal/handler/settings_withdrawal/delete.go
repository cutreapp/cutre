package settings_withdrawal

import (
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Delete DELETE /settings/withdrawal - 今のパスワードを確かめて退会させ、Cookieを消してトップページへ送る。
//
// フォームはPOSTに _method を載せて届く。退会のUseCaseがユーザーのすべてのセッションの行を消すため、
// このブラウザに残ったセッションのCookieもログアウトと同じく消す。
// フォームを受け付けなかったときは、退会の画面をメッセージ付きで描き直す。
// 二重送信で先の送信が退会させていたときは、完了のメッセージを出さずに同じ行き先へ送る。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の退会かを決められないまま進めない。
		slog.ErrorContext(ctx, "退会に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	confirmed := r.PostFormValue("confirmed") != ""

	resetAt, err := h.limiter.CheckWithdrawal(ctx, user.ID.String())
	if err != nil {
		slog.ErrorContext(ctx, "退会のレート制限の判定に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		untilReset := time.Until(resetAt)
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(untilReset.Seconds()))))
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_totp_rate_limited", map[string]any{"Minutes": ratelimit.WaitMinutes(untilReset)}))
		h.renderNew(w, r, user, http.StatusTooManyRequests, confirmed, ve)
		return
	}

	err = h.deleteAccountUC.Execute(ctx, usecase.DeleteAccountInput{
		User:            user,
		CurrentPassword: r.PostFormValue("current_password"),
		Confirmed:       confirmed,
	})
	// 行き先は、退会した利用者の言語のトップページにする (ログアウトと同じ)。
	// メッセージは UserLocale が users.locale に切り替えたctxの言語で出る。
	locale := string(user.Locale)
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, confirmed, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			h.sessionMgr.DeleteSessionCookie(w)
			http.Redirect(w, r, i18n.LocalePath(locale, "/"), http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "退会に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.DeleteSessionCookie(w)
	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_account_withdrawn"))
	http.Redirect(w, r, i18n.LocalePath(locale, "/"), http.StatusSeeOther)
}
