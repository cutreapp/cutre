// Package admin_event_category は管理画面のカテゴリーの作成・編集・削除のハンドラーを提供する。
// カテゴリーの一覧はイベントの編集の画面 (admin_event) に並べる。
package admin_event_category

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

// Handler は管理画面のカテゴリーのHTTPハンドラー。
type Handler struct {
	cfg                     *config.Config
	errorRenderer           *httperror.Renderer
	flashMgr                *session.FlashManager
	getAdminEventUC         *usecase.GetAdminEventUsecase
	getAdminEventCategoryUC *usecase.GetAdminEventCategoryUsecase
	createEventCategoryUC   *usecase.CreateEventCategoryUsecase
	updateEventCategoryUC   *usecase.UpdateEventCategoryUsecase
	deleteEventCategoryUC   *usecase.DeleteEventCategoryUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminEventUC *usecase.GetAdminEventUsecase,
	getAdminEventCategoryUC *usecase.GetAdminEventCategoryUsecase,
	createEventCategoryUC *usecase.CreateEventCategoryUsecase,
	updateEventCategoryUC *usecase.UpdateEventCategoryUsecase,
	deleteEventCategoryUC *usecase.DeleteEventCategoryUsecase,
) *Handler {
	return &Handler{
		cfg:                     cfg,
		errorRenderer:           errorRenderer,
		flashMgr:                flashMgr,
		getAdminEventUC:         getAdminEventUC,
		getAdminEventCategoryUC: getAdminEventCategoryUC,
		createEventCategoryUC:   createEventCategoryUC,
		updateEventCategoryUC:   updateEventCategoryUC,
		deleteEventCategoryUC:   deleteEventCategoryUC,
	}
}

// eventIDFromURL は作成の画面と作成のURLの {id} を、カテゴリーを作成するイベントのIDとして読む。
// UUIDとして読めないときはfalseを返す。
func eventIDFromURL(r *http.Request) (model.EventID, bool) {
	return httpadmin.ParseIDParam[model.EventID](r, "id")
}

// eventCategoryIDFromURL は編集・更新・削除のURLの {id} をカテゴリーのIDとして読む。UUIDとして読めないときはfalseを返す。
func eventCategoryIDFromURL(r *http.Request) (model.EventCategoryID, bool) {
	return httpadmin.ParseIDParam[model.EventCategoryID](r, "id")
}

// respondError は、UseCaseのエラーのうち個別に扱わなかったものに応える。
//
// 管理画面を使えないときとカテゴリーやイベントが無いときは、存在しないページとして404を返す (httpadmin.IsNotFound)。
// それ以外は500で応える。
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	if httpadmin.IsNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	slog.ErrorContext(r.Context(), logMessage, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
