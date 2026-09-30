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

// Create POST /settings/message_consent - メッセージの取り扱いの今の文面に同意し、完了のメッセージを付けてメッセージの利用の画面へ戻す。
//
// 既に有効な同意があったとき (二重送信や別の画面で先に同意した) は、そのままメッセージの利用の画面へ戻す。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の同意かを決められないまま記録しない。
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.createMessageConsentUC.Execute(ctx, usecase.CreateMessageConsentInput{UserID: user.ID}); err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
			return
		}
		slog.ErrorContext(ctx, "メッセージの取り扱いへの同意に失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_message_consent_agreed"))
	http.Redirect(w, r, templates.SettingsMessageConsentPath, http.StatusSeeOther)
}
