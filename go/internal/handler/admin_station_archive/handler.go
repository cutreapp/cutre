// Package admin_station_archive は管理画面の駅のアーカイブのハンドラーを提供する。
// 理由の入力 (GET)、アーカイブ (POST)、公開に戻す (DELETE) を扱う。
package admin_station_archive

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

// Handler は管理画面の駅のアーカイブのHTTPハンドラー。
type Handler struct {
	cfg                *config.Config
	errorRenderer      *httperror.Renderer
	flashMgr           *session.FlashManager
	getAdminStationUC  *usecase.GetAdminStationUsecase
	archiveStationUC   *usecase.ArchiveStationUsecase
	unarchiveStationUC *usecase.UnarchiveStationUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminStationUC *usecase.GetAdminStationUsecase,
	archiveStationUC *usecase.ArchiveStationUsecase,
	unarchiveStationUC *usecase.UnarchiveStationUsecase,
) *Handler {
	return &Handler{
		cfg:                cfg,
		errorRenderer:      errorRenderer,
		flashMgr:           flashMgr,
		getAdminStationUC:  getAdminStationUC,
		archiveStationUC:   archiveStationUC,
		unarchiveStationUC: unarchiveStationUC,
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
