// Package item はリストのアイテムのハンドラーを提供する。
// イベントのグッズを、譲れる・ほしいのリストに追加する画面 (GET) と追加 (POST) と、
// リストにあるアイテムの編集の画面 (GET)・更新 (PATCH)・リストから外す (DELETE) を扱う。
package item

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はリストのアイテムのHTTPハンドラー。
type Handler struct {
	cfg           *config.Config
	errorRenderer *httperror.Renderer
	flashMgr      *session.FlashManager
	getGoodsUC    *usecase.GetGoodsUsecase
	createItemUC  *usecase.CreateItemUsecase
	getItemUC     *usecase.GetItemUsecase
	updateItemUC  *usecase.UpdateItemUsecase
	deleteItemUC  *usecase.DeleteItemUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getGoodsUC *usecase.GetGoodsUsecase,
	createItemUC *usecase.CreateItemUsecase,
	getItemUC *usecase.GetItemUsecase,
	updateItemUC *usecase.UpdateItemUsecase,
	deleteItemUC *usecase.DeleteItemUsecase,
) *Handler {
	return &Handler{
		cfg:           cfg,
		errorRenderer: errorRenderer,
		flashMgr:      flashMgr,
		getGoodsUC:    getGoodsUC,
		createItemUC:  createItemUC,
		getItemUC:     getItemUC,
		updateItemUC:  updateItemUC,
		deleteItemUC:  deleteItemUC,
	}
}

// parseGoodsID は、クエリやフォームの goods_id をグッズのIDとして読む。UUIDとして読めないときはfalseを返す。
func parseGoodsID(raw string) (model.GoodsID, bool) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return model.GoodsID{}, false
	}

	return model.GoodsID(parsed), true
}

// itemIDFromURL は、URLの {id} をアイテムのIDとして読む。UUIDとして読めないときはfalseを返す。
func itemIDFromURL(r *http.Request) (model.ItemID, bool) {
	parsed, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return model.ItemID{}, false
	}

	return model.ItemID(parsed), true
}

// isNotFound は、UseCaseのエラーが、存在しないページとして404で応えるものかを返す。
// グッズが無いか公開中でないときと、アイテムが無いか、リストから外したものか、ほかのユーザーのものであるときに当たる。
func isNotFound(err error) bool {
	ae := model.AsAppError(err)
	return ae != nil && ae.Code == model.AppErrCodeResourceNotFound
}

// respondGetGoodsError は、グッズの取得のエラーに応える。
// グッズが無いか公開中でないときは404を、それ以外は500を返す。
func (h *Handler) respondGetGoodsError(w http.ResponseWriter, r *http.Request, err error) {
	if isNotFound(err) {
		h.errorRenderer.NotFound(w, r)
		return
	}

	h.respondInternalError(w, r, err, "リストに追加するグッズの取得に失敗しました")
}
