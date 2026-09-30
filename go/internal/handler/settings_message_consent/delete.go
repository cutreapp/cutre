package settings_message_consent

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Delete DELETE /settings/message_consent - メッセージの取り扱いへの同意をやめ、完了のメッセージを付けてメッセージの利用の画面へ戻す。
//
// フォームはPOSTに _method を載せて届く。
// 進行中の交換があるときはやめず、そのことを伝えてメッセージの利用の画面を描き直す (422)。
// やめていない同意が無かったとき (二重送信や別の画面で先にやめた) は、そのままメッセージの利用の画面へ戻す。
// もう一度同意すれば元に戻せるため、確認のダイアログは挟まない。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の同意かを決められないままやめない。
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意の取りやめに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.withdrawMessageConsentUC.Execute(ctx, usecase.WithdrawMessageConsentInput{UserID: user.ID}); err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderShow(w, r, user, http.StatusUnprocessableEntity, ve)
			return
		}
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意の取りやめに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_message_consent_withdrawn"))
	http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
}
