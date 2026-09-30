package admin_station_archive

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

// Create POST /admin/stations/{id}/archive - 理由を残して駅をアーカイブし、完了のメッセージを付けて編集の画面へ戻す。
//
// 理由を受け付けなかったときは、送られた理由とエラーを付けてアーカイブの画面を描き直す (422)。
// 画面を開いたあとにほかの操作で更新されていたときは、最新の状態と競合を示す (409)。
// 管理画面を使えないユーザーと、無い駅・削除した駅には404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないままアーカイブしない。
		slog.ErrorContext(ctx, "駅のアーカイブに現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	stationID, ok := stationIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}
	// 行き先はUUIDとして読んだIDから組み立てた、このサイトの中のパスに限られる。
	// gosecのG710 (オープンリダイレクト) はURLから読んだ値の検証を追えずに警告するため、下のリダイレクトでは誤検知。
	editPath := templates.EditAdminStationPath(stationID.String())

	archiveMessage := r.PostFormValue("archive_message")
	err := h.archiveStationUC.Execute(ctx, usecase.ArchiveStationInput{User: user, StationID: stationID, LockVersion: httpform.LockVersion(r), ArchiveMessage: archiveMessage})
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.AppErrCodeConflict {
			h.renderConflict(w, r, user, stationID)
			return
		}
		if ve := model.AsValidationError(err); ve != nil {
			output, getErr := h.getAdminStationUC.Execute(ctx, usecase.GetAdminStationInput{User: user, StationID: stationID})
			if getErr != nil {
				h.respondError(w, r, getErr, "管理画面の駅の取得に失敗しました")
				return
			}
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, output, archiveMessage, ve)
			return
		}
		h.respondError(w, r, err, "駅のアーカイブに失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_station_archived"))
	//nolint:gosec // G710
	http.Redirect(w, r, editPath, http.StatusSeeOther)
}
