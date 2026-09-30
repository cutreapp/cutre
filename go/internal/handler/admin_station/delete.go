package admin_station

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/httpadmin"
	"github.com/cutreapp/cutre/go/internal/httpform"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Delete DELETE /admin/stations/{id} - 駅を削除し、完了のメッセージを付けて駅の一覧へ戻す。
//
// フォームは確認のダイアログからPOSTに _method を載せて届く。
// 削除は管理者だけができ、それ以外のユーザーと、無い駅・削除した駅には404を返す。
// 交換場所に選んでいるユーザーがいるときは、アーカイブを案内するエラーを付けて編集の画面を描き直す (422)。
// 画面を開いたあとにほかの操作で更新されていたときは、最新の値で編集の画面を描き直す (409)。
// CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま削除しない。
		slog.ErrorContext(ctx, "駅の削除に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	stationID, ok := stationIDFromURL(r)
	if !ok {
		h.errorRenderer.NotFound(w, r)
		return
	}

	if err := h.deleteStationUC.Execute(ctx, usecase.DeleteStationInput{User: user, StationID: stationID, LockVersion: httpform.LockVersion(r)}); err != nil {
		status, formErrors := httpadmin.DeleteFailure(err, i18n.T(ctx, "admin_station_conflict_message"))
		if formErrors == nil {
			h.respondError(w, r, err, "駅の削除に失敗しました")
			return
		}
		output, getErr := h.getAdminStationUC.Execute(ctx, usecase.GetAdminStationInput{User: user, StationID: stationID})
		if getErr != nil {
			h.respondError(w, r, getErr, "管理画面の駅の取得に失敗しました")
			return
		}
		h.renderEdit(w, r, user, status, output, viewmodel.NewStationForm(output.Station), formErrors)
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_station_deleted"))
	http.Redirect(w, r, templates.AdminStationsPath, http.StatusSeeOther)
}
