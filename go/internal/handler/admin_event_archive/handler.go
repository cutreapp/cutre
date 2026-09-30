// Package admin_event_archive は管理画面のイベントのアーカイブのハンドラーを提供する。
// 理由の入力 (GET)、アーカイブ (POST)、公開に戻す (DELETE) を扱う。
package admin_event_archive

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

// Handler は管理画面のイベントのアーカイブのHTTPハンドラー。
type Handler struct {
	cfg              *config.Config
	errorRenderer    *httperror.Renderer
	flashMgr         *session.FlashManager
	getAdminEventUC  *usecase.GetAdminEventUsecase
	archiveEventUC   *usecase.ArchiveEventUsecase
	unarchiveEventUC *usecase.UnarchiveEventUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminEventUC *usecase.GetAdminEventUsecase,
	archiveEventUC *usecase.ArchiveEventUsecase,
	unarchiveEventUC *usecase.UnarchiveEventUsecase,
) *Handler {
	return &Handler{
		cfg:              cfg,
		errorRenderer:    errorRenderer,
		flashMgr:         flashMgr,
		getAdminEventUC:  getAdminEventUC,
		archiveEventUC:   archiveEventUC,
		unarchiveEventUC: unarchiveEventUC,
	}
}

// eventIDFromURL はURLの {id} をイベントのIDとして読む。UUIDとして読めないときはfalseを返す。
func eventIDFromURL(r *http.Request) (model.EventID, bool) {
	return httpadmin.ParseIDParam[model.EventID](r, "id")
}

// respondError は、UseCaseのエラーのうち個別に扱わなかったものに応える。
//
// 管理画面を使えないときとイベントが無いときは、存在しないページとして404を返す (httpadmin.IsNotFound)。
// それ以外は500で応える。
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	if httpadmin.IsNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	slog.ErrorContext(r.Context(), logMessage, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
