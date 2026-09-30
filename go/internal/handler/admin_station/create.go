package admin_station

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/middleware"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/templates"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/viewmodel"
)

// Create POST /admin/stations - 駅を作成し、完了のメッセージを付けて駅の一覧へ戻す。
//
// フォームを受け付けなかったときは、送られた値とエラーを付けて作成の画面を描き直す (422)。
// 管理画面を使えないユーザーには404を返す。CSRFの検証は上流のミドルウェアが済ませている。
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user := middleware.UserFromContext(ctx)
	if user == nil {
		// RequireAuth を通していない配線の誤り。誰の操作かを決められないまま作成しない。
		slog.ErrorContext(ctx, "駅の作成に現在のユーザーがありません (RequireAuth を通していません)")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	form := viewmodel.StationForm{
		PrefectureCode: r.PostFormValue("prefecture_code"),
		Name:           r.PostFormValue("name"),
		Position:       r.PostFormValue("position"),
	}
	_, err := h.createStationUC.Execute(ctx, usecase.CreateStationInput{
		User:           user,
		PrefectureCode: form.PrefectureCode,
		Name:           form.Name,
		Position:       form.Position,
	})
	if err != nil {
		if ve := model.AsValidationError(err); ve != nil {
			h.renderNew(w, r, user, http.StatusUnprocessableEntity, form, ve)
			return
		}
		h.respondError(w, r, err, "駅の作成に失敗しました")
		return
	}

	h.flashMgr.SetSuccess(w, i18n.T(ctx, "flash_station_created"))
	http.Redirect(w, r, templates.AdminStationsPath, http.StatusSeeOther)
}
