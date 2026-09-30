// Package trade_decline は、交換の申し込みのお断りのハンドラーを提供する。
// 理由とひとことを入れる画面 (GET) と、お断り (POST) を扱う。
package trade_decline

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

// Handler は申し込みのお断りのHTTPハンドラー。
type Handler struct {
	cfg            *config.Config
	errorRenderer  *httperror.Renderer
	flashMgr       *session.FlashManager
	getTradeUC     *usecase.GetTradeUsecase
	declineTradeUC *usecase.DeclineTradeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getTradeUC *usecase.GetTradeUsecase,
	declineTradeUC *usecase.DeclineTradeUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, flashMgr: flashMgr, getTradeUC: getTradeUC, declineTradeUC: declineTradeUC}
}

// tradeIDFromURL はパスの {id} を交換のIDとして読む。UUIDとして読めなければfalseを返す。
func tradeIDFromURL(r *http.Request) (model.TradeID, bool) {
	parsed, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return model.TradeID{}, false
	}

	return model.TradeID(parsed), true
}
