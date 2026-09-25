package settings_two_factor_auth

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/qrcode"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/templates/components"
	"github.com/cutreapp/cutre/go/internal/templates/layouts"
	page "github.com/cutreapp/cutre/go/internal/templates/pages/settings_two_factor_auth"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// New GET /settings/two_factor_auth/new - 認証アプリへ登録する新しい秘密鍵を作り、登録の画面を描画する。
// ログインは RequireAuth が求め、表示言語は UserLocale が users.locale に切り替えてから届く。
//
// 既に有効にしているときは、秘密鍵を差し替えずに二要素認証の画面へ送る。
// GETで行を書き換えることになるが、ログイン後の no-store の画面でプリフェッチやクローラーが届かず、
// 書き換えるのは有効にする前の秘密鍵だけのため許容する。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の秘密鍵かを決められないまま作らない。
		slog.ErrorContext(ctx, "二要素認証の登録の画面に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	output, err := h.prepareTwoFactorAuthUC.Execute(ctx, usecase.PrepareTwoFactorAuthInput{User: user})
	if err != nil {
		slog.ErrorContext(ctx, "二要素認証の登録の準備に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if output.Setup == nil {
		http.Redirect(w, r, templates.SettingsTwoFactorAuthPath, http.StatusSeeOther)
		return
	}

	h.renderNew(w, r, user, http.StatusOK, output.Setup, "", nil)
}

// renderNew は登録の画面を指定したステータスで描画する。
// 表示 (200) と、コードを受け付けなかったときの再描画 (422) で共有する。
func (h *Handler) renderNew(
	w http.ResponseWriter,
	r *http.Request,
	user *model.User,
	status int,
	setup *usecase.TwoFactorAuthSetup,
	code string,
	formErrors *model.ValidationError,
) {
	ctx := r.Context()

	qrCode, err := qrcode.Encode(setup.OTPAuthURL)
	if err != nil {
		slog.ErrorContext(ctx, "二要素認証の登録のQRコードの作成に失敗しました", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	meta := viewmodel.SignedInPageMeta(ctx, h.cfg)
	meta.SetTitle(ctx, "settings_two_factor_auth_new_title")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	layoutData := layouts.DefaultLayoutData{
		Meta:   meta,
		Header: &components.HeaderData{Atname: user.Atname, CurrentPath: templates.NewSettingsTwoFactorAuthPath},
	}
	data := page.NewPageData{
		CSRFToken:  middleware.CSRFTokenFromContext(ctx),
		Secret:     setup.Secret,
		OTPAuthURL: setup.OTPAuthURL,
		QRCode:     qrCode,
		Code:       code,
		FormErrors: formErrors,
	}
	if err := layouts.Default(layoutData, page.New(data)).Render(ctx, w); err != nil {
		// ステータスとヘッダーは送出済みのため、500には変えられずログに残すだけになる。
		slog.ErrorContext(ctx, "二要素認証の登録の画面の描画に失敗しました", "error", err)
	}
}
