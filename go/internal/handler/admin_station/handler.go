// Package admin_station は管理画面の駅の一覧・作成・編集・削除のハンドラーを提供する。
package admin_station

import (
	"log/slog"
	"net/http"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httpadmin"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は管理画面の駅のHTTPハンドラー。
type Handler struct {
	cfg                *config.Config
	errorRenderer      *httperror.Renderer
	flashMgr           *session.FlashManager
	getAdminStationsUC *usecase.GetAdminStationsUsecase
	getAdminStationUC  *usecase.GetAdminStationUsecase
	createStationUC    *usecase.CreateStationUsecase
	updateStationUC    *usecase.UpdateStationUsecase
	deleteStationUC    *usecase.DeleteStationUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminStationsUC *usecase.GetAdminStationsUsecase,
	getAdminStationUC *usecase.GetAdminStationUsecase,
	createStationUC *usecase.CreateStationUsecase,
	updateStationUC *usecase.UpdateStationUsecase,
	deleteStationUC *usecase.DeleteStationUsecase,
) *Handler {
	return &Handler{
		cfg:                cfg,
		errorRenderer:      errorRenderer,
		flashMgr:           flashMgr,
		getAdminStationsUC: getAdminStationsUC,
		getAdminStationUC:  getAdminStationUC,
		createStationUC:    createStationUC,
		updateStationUC:    updateStationUC,
		deleteStationUC:    deleteStationUC,
	}
}

// stationIDFromURL はURLの {id} を駅のIDとして読む。UUIDとして読めないときはfalseを返す。
func stationIDFromURL(r *http.Request) (model.StationID, bool) {
	return httpadmin.ParseIDParam[model.StationID](r, "id")
}

// respondError は、UseCaseのエラーのうち個別に扱わなかったものに応える。
//
// 管理画面を使えないときと駅が無いときは、存在しないページとして404を返す (httpadmin.IsNotFound)。
// それ以外は500で応える。
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	if httpadmin.IsNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	slog.ErrorContext(r.Context(), logMessage, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
