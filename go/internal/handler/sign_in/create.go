package sign_in

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
	signinpage "github.com/cutreapp/cutre/go/internal/templates/pages/sign_in"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /sign_in (日本語版) と POST /en/sign_in (英語版) - 資格情報を照合し、一致すればセッションを発行する。
// 二要素認証を有効にしたユーザーは、セッションを発行せずに認証アプリのコードの入力画面へ送る。
//
// 受け付けなかった送信はフォームをメッセージ付きで再描画する。メールアドレスは戻し、パスワードは戻さない。
// CSRFの検証は上流のミドルウェアが済ませている。
//
// Bot対策とレート制限は、フォームの内容によらないリクエスト単位の関門としてUseCaseより前に置く。
// どちらかで止めた送信は資格情報を照合しないため、パスワードの総当たりの手がかりを与えない。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	email := r.PostFormValue("email")
	password := r.PostFormValue("password")
	// ログイン後の行き先はフォームから届くため、リダイレクト先に使う前に検証する。
	returnTo := middleware.SanitizeReturnTo(r.PostFormValue(templates.ReturnToParam))

	rerender := func(status int, formErrors *model.ValidationError) {
		h.render(w, r, status, signinpage.NewPageData{
			CSRFToken:  middleware.CSRFTokenFromContext(ctx),
			Email:      email,
			FormErrors: formErrors,
			ReturnTo:   returnTo,
		})
	}

	passed, err := h.turnstile.Verify(ctx, r.PostFormValue("cf-turnstile-response"))
	if err != nil || !passed {
		// 単なる非通過 (トークンが空) はBotの想定内の拒否のため、errorがあるときだけ添える。
		attrs := []any{}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		slog.WarnContext(ctx, "Turnstileの確認を通過しなかったため、ログインを受け付けません", attrs...)
		rerender(http.StatusUnprocessableEntity, globalError(ctx, "validation_turnstile_failed"))
		return
	}

	ip := clientip.Resolve(r, h.cfg.TrustedProxies)

	resetAt, err := h.checkRateLimit(ctx, ip, email)
	if err != nil {
		slog.ErrorContext(ctx, "ログインのレート制限の判定に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !resetAt.IsZero() {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(time.Until(resetAt).Seconds()))))
		rerender(http.StatusTooManyRequests, globalError(ctx, "validation_rate_limited"))
		return
	}

	signInOutput, err := h.createSignInUC.Execute(ctx, usecase.CreateSignInInput{
		Email:    email,
		Password: password,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			rerender(http.StatusUnprocessableEntity, ve)
			return
		}
		slog.ErrorContext(ctx, "ログインの資格情報の照合に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user := signInOutput.User

	// 二要素認証を有効にしたユーザーには、まだセッションを発行しない。
	// パスワードを確かめたことを署名付きのCookieで運び、コードが合ったときにセッションと引き換える。
	if signInOutput.TwoFactorAuthRequired {
		h.continuationMgr.SetTwoFactorPendingUserID(w, user.ID)
		destination := templates.WithReturnTo(templates.LocalePath(ctx, templates.SignInTwoFactorPath), returnTo)
		// 戻り先は SanitizeReturnTo で同一オリジンの相対パスに絞ってからクエリへ載せており、行き先はコードの入力画面に固定されている。
		//nolint:gosec // G710
		http.Redirect(w, r, destination, http.StatusSeeOther)
		return
	}

	sessionOutput, err := h.createSessionUC.Execute(ctx, usecase.CreateSessionInput{
		UserID:    user.ID,
		IPAddress: ip,
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		// 資格情報は一致したが、セッションを作れなかった。黙って未ログインのままにせず500にする。
		slog.ErrorContext(ctx, "セッションの作成に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.sessionMgr.SetSessionCookie(w, sessionOutput.Token)

	// 行き先のページは users.locale の言語で表示されるため、メッセージもその言語にする。
	// ログイン画面の言語で書くと、英語版からログインした日本語の利用者に英語のメッセージが出る。
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

// checkRateLimit はこの送信をIPアドレスとメールアドレスの両方の単位で数え、どちらかが上限を超えていれば
// 数え直しになる時刻を返す。どちらも上限内ならゼロ値を返す。
//
// 片方が超えていても両方を数える。数えるのを止めると、止めた側の単位で攻撃の規模を後から把握できなくなるため。
// メールアドレスが空の送信はアカウントを指さないため、IPアドレスの単位だけで数える。
func (h *Handler) checkRateLimit(ctx context.Context, ip, email string) (time.Time, error) {
	type unitCheck struct {
		unit  string
		input ratelimit.CheckInput
	}
	checks := []unitCheck{
		{unit: "ip", input: ratelimit.CheckInput{Key: ratelimit.IPKey(rateLimitAction, ip), Limit: ipRateLimit, Window: rateLimitWindow}},
	}
	if email != "" {
		checks = append(checks, unitCheck{unit: "email", input: ratelimit.CheckInput{Key: ratelimit.EmailKey(rateLimitAction, email), Limit: emailRateLimit, Window: rateLimitWindow}})
	}

	var resetAt time.Time
	for _, check := range checks {
		result, err := h.limiter.Check(ctx, check.input)
		if err != nil {
			return time.Time{}, err
		}
		if !result.Allowed {
			// メールアドレスはログに出さず、どの単位で止めたかだけを残す。
			slog.WarnContext(ctx, "レート制限の上限を超えたため、ログインを受け付けません", "unit", check.unit, "ip", ip, "count", result.Count)
			if result.ResetAt.After(resetAt) {
				resetAt = result.ResetAt
			}
		}
	}
	return resetAt, nil
}

// globalError はフォーム全体に関わる1件のエラーを返す。
func globalError(ctx context.Context, messageID string) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, messageID))
	return ve
}
