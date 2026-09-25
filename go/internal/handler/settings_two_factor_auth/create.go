package settings_two_factor_auth

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Create POST /settings/two_factor_auth - 認証アプリのコードを照合して二要素認証を有効にし、リカバリーコードを描画する。
//
// リカバリーコードは保存していないため、リダイレクトせずにこの応答として一度だけ描く。
// コードの誤りでは、同じ秘密鍵のまま登録の画面を描き直す。認証アプリへ登録し直させないため。
// 登録の途中の設定が無いとき (二重送信で既に有効にした・別の画面で秘密鍵を作り直した) は、二要素認証の画面へ送る。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の設定かを決められないまま有効にしない。
		slog.ErrorContext(ctx, "二要素認証の有効化に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	code := r.PostFormValue("code")
	output, err := h.enableTwoFactorAuthUC.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: code})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.rerenderNew(w, r, user, code, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			http.Redirect(w, r, templates.SettingsTwoFactorAuthPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "二要素認証の有効化に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_two_factor_auth_create_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	layoutData := layouts.DefaultLayoutData{
		Meta:   meta,
		Header: &components.HeaderData{Atname: user.Atname, CurrentPath: templates.SettingsTwoFactorAuthPath},
	}
	if err := layouts.Default(layoutData, page.Create(page.CreatePageData{RecoveryCodes: output.RecoveryCodes})).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "リカバリーコードの画面の描画に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// rerenderNew は、受け付けなかったコードを入力欄に戻し、登録の途中の秘密鍵のまま登録の画面を422で描き直す。
// 登録の途中の設定が無くなっていたときは、二要素認証の画面へ送る。
func (h *Handler) rerenderNew(w http.ResponseWriter, r *http.Request, user *model.User, code string, formErrors *model.ValidationError) {
	ctx := r.Context()

	pending, err := h.getPendingTwoFactorAuthUC.Execute(ctx, usecase.GetPendingTwoFactorAuthInput{User: user})
	if err != nil {
		slog.ErrorContext(ctx, "登録の途中の二要素認証の設定の取得に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if pending.Setup == nil {
		http.Redirect(w, r, templates.SettingsTwoFactorAuthPath, http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, user, http.StatusUnprocessableEntity, pending.Setup, code, formErrors)
}
