// Package admin_event は管理画面のイベントの一覧・作成・編集・削除のハンドラーを提供する。
package admin_event

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

// Handler は管理画面のイベントのHTTPハンドラー。
//
// 依存は8つを超えるが、どれも /admin/events 配下の1つのリソースの操作に要るもので、
// 分けると一覧・作成・編集のハンドラーが同じURLの階層をまたいで散らばるため、1つにまとめている。
type Handler struct {
	cfg              *config.Config
	errorRenderer    *httperror.Renderer
	flashMgr         *session.FlashManager
	getAdminMenuUC   *usecase.GetAdminMenuUsecase
	getAdminEventsUC *usecase.GetAdminEventsUsecase
	getAdminEventUC  *usecase.GetAdminEventUsecase
	createEventUC    *usecase.CreateEventUsecase
	updateEventUC    *usecase.UpdateEventUsecase
	deleteEventUC    *usecase.DeleteEventUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminMenuUC *usecase.GetAdminMenuUsecase,
	getAdminEventsUC *usecase.GetAdminEventsUsecase,
	getAdminEventUC *usecase.GetAdminEventUsecase,
	createEventUC *usecase.CreateEventUsecase,
	updateEventUC *usecase.UpdateEventUsecase,
	deleteEventUC *usecase.DeleteEventUsecase,
) *Handler {
	return &Handler{
		cfg:              cfg,
		errorRenderer:    errorRenderer,
		flashMgr:         flashMgr,
		getAdminMenuUC:   getAdminMenuUC,
		getAdminEventsUC: getAdminEventsUC,
		getAdminEventUC:  getAdminEventUC,
		createEventUC:    createEventUC,
		updateEventUC:    updateEventUC,
		deleteEventUC:    deleteEventUC,
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
