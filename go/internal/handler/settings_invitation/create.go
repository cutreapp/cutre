package settings_invitation

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /settings/invitation - 招待リンクを作り直し、招待の画面へ戻す。
//
// 作り直さなかったとき (人数の上限に達していた、または表示した招待が既に作り直されていた) は、完了のメッセージを出さずに戻す。
// 招待の画面が上限に達した旨や今の招待を示す。
// 招待IDが不正なときと他人の招待IDのときは、招待の有無を明かさないよう、どちらも404で応える。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の招待かを決められないまま作り直さない。
		slog.ErrorContext(ctx, "招待リンクの作り直しに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rawID := r.PostFormValue("invitation_id")
	parsed, err := uuid.Parse(rawID)
	if err != nil {
		h.errorRenderer.NotFound(w, r)
		return
	}

	output, err := h.recreateInvitationUC.Execute(ctx, usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: model.InvitationID(parsed)})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeForbidden {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "招待リンクの作り直しに失敗しました", "error", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if output.Recreated {
		h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_invitation_recreated"))
	}
	http.Redirect(w, r, templates.SettingsInvitationPath, http.StatusSeeOther)
}
