// Package admin_goods は管理画面のグッズの作成・編集・削除のハンドラーを提供する。
// グッズの一覧はカテゴリーの編集の画面 (admin_event_category) に並べる。
package admin_goods

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

// Handler は管理画面のグッズのHTTPハンドラー。
type Handler struct {
	cfg                     *config.Config
	errorRenderer           *httperror.Renderer
	flashMgr                *session.FlashManager
	getAdminEventCategoryUC *usecase.GetAdminEventCategoryUsecase
	getAdminGoodsUC         *usecase.GetAdminGoodsUsecase
	createGoodsUC           *usecase.CreateGoodsUsecase
	updateGoodsUC           *usecase.UpdateGoodsUsecase
	deleteGoodsUC           *usecase.DeleteGoodsUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getAdminEventCategoryUC *usecase.GetAdminEventCategoryUsecase,
	getAdminGoodsUC *usecase.GetAdminGoodsUsecase,
	createGoodsUC *usecase.CreateGoodsUsecase,
	updateGoodsUC *usecase.UpdateGoodsUsecase,
	deleteGoodsUC *usecase.DeleteGoodsUsecase,
) *Handler {
	return &Handler{
		cfg:                     cfg,
		errorRenderer:           errorRenderer,
		flashMgr:                flashMgr,
		getAdminEventCategoryUC: getAdminEventCategoryUC,
		getAdminGoodsUC:         getAdminGoodsUC,
		createGoodsUC:           createGoodsUC,
		updateGoodsUC:           updateGoodsUC,
		deleteGoodsUC:           deleteGoodsUC,
	}
}

// eventCategoryIDFromURL は作成の画面と作成のURLの {id} を、グッズを作成するカテゴリーのIDとして読む。
// UUIDとして読めないときはfalseを返す。
func eventCategoryIDFromURL(r *http.Request) (model.EventCategoryID, bool) {
	return httpadmin.ParseIDParam[model.EventCategoryID](r, "id")
}

// goodsIDFromURL は編集・更新・削除のURLの {id} をグッズのIDとして読む。UUIDとして読めないときはfalseを返す。
func goodsIDFromURL(r *http.Request) (model.GoodsID, bool) {
	return httpadmin.ParseIDParam[model.GoodsID](r, "id")
}

// respondError は、UseCaseのエラーのうち個別に扱わなかったものに応える。
//
// 管理画面を使えないときとグッズ・カテゴリー・イベントが無いときは、存在しないページとして404を返す (httpadmin.IsNotFound)。
// それ以外は500で応える。
func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error, logMessage string) {
	if httpadmin.IsNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	slog.ErrorContext(r.Context(), logMessage, "error", err)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}
