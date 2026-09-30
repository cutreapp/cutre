package admin_event_archive

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Create POST /admin/events/{id}/archive - 理由を残してイベントをアーカイブし、完了のメッセージを付けて編集の画面へ戻す。
//
// 理由を受け付けなかったときは、送られた理由とエラーを付けてアーカイブの画面を描き直す (422)。
// 画面を開いたあとにほかの操作で更新されていたときは、最新の状態と競合を示す (409)。
// 管理画面を使えないユーザーと、無いイベント・削除したイベントには404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないままアーカイブしない。
		slog.ErrorContext(ctx, "イベントのアーカイブに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	eventID, ok := eventIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	editPath := templates.EditAdminEventPath(eventID.String())

	archiveMessage := r.PostFormValue("archive_message")
	err := h.archiveEventUC.Execute(ctx, usecase.ArchiveEventInput{User: user, EventID: eventID, LockVersion: httpform.LockVersion(r), ArchiveMessage: archiveMessage})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			h.renderConflict(w, r, user, eventID)
			return
		}
		if ve := model.AsValidationError(err); ve != nil {
			output, getErr := h.getAdminEventUC.Execute(ctx, usecase.GetAdminEventInput{User: user, EventID: eventID})
			if getErr != nil {
				h.respondError(w, r, getErr, "管理画面のイベントの取得に失敗しました")
				return
			}
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, output.Event, archiveMessage, ve)
			return
		}
		h.respondError(w, r, err, "イベントのアーカイブに失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_event_archived"))
	//nolint:gosec // G710
	http.Redirect(w, r, editPath, http.StatusSeeOther)
}
