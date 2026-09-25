package settings_two_factor_auth

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Show GET /settings/two_factor_auth - 二要素認証の状態を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の設定かを決められないまま描画しない。
		slog.ErrorContext(ctx, "二要素認証の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.renderShow(w, r, user, http.StatusOK, nil)
}

// renderShow は二要素認証の画面を指定したステータスで描画する。
// 無効にするフォームの再認証を受け付けなかったとき (422・429) にも、今の状態と一緒にエラーを描き直すのに使う。
func (h *Handler) renderShow(w http.ResponseWriter, r *http.Request, user *model.User, status int, formErrors *model.ValidationError) {
	ctx := r.Context()

	twoFactorStatus, err := h.getTwoFactorAuthStatusUC.Execute(ctx, usecase.GetTwoFactorAuthStatusInput{UserID: user.ID})
	if err != nil {
		slog.ErrorContext(ctx, "二要素認証の状態の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := page.ShowPageData{
		ProfilePath:             templates.ProfilePath(user.Atname),
		Enabled:                 twoFactorStatus.TwoFactorAuth != nil,
		UnusedRecoveryCodeCount: twoFactorStatus.UnusedRecoveryCodeCount,
		CSRFToken:               middleware.CSRFTokenFromContext(ctx),
		FormErrors:              formErrors,
	}
	if data.Enabled {
		// 有効にした日はユーザーのタイムゾーンの日付で示す。
		loc, err := user.Location()
		if err != nil {
			slog.ErrorContext(ctx, "ユーザーのタイムゾーンの読み込みに失敗しました", "error", err, "user_id", user.ID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		data.EnabledOn = viewmodel.FormatDate(ctx, twoFactorStatus.TwoFactorAuth.EnabledAt.In(loc), time.Now().In(loc))
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_two_factor_auth_show_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:   meta,
		Header: &components.HeaderData{Atname: user.Atname, CurrentPath: templates.SettingsTwoFactorAuthPath},
	}
	if err := layouts.Default(layoutData, page.Show(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "二要素認証の画面の描画に失敗しました", "error", err)
	}
}
