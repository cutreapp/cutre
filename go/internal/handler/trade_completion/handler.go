// Package trade_completion は、交換の「交換できた」のハンドラーを提供する。
// ひとことを入れる画面 (GET) と、記録 (POST) を扱う。
package trade_completion

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

// Handler は「交換できた」のHTTPハンドラー。
type Handler struct {
	cfg             *config.Config
	errorRenderer   *httperror.Renderer
	flashMgr        *session.FlashManager
	getTradeUC      *usecase.GetTradeUsecase
	completeTradeUC *usecase.CompleteTradeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getTradeUC *usecase.GetTradeUsecase,
	completeTradeUC *usecase.CompleteTradeUsecase,
) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, flashMgr: flashMgr, getTradeUC: getTradeUC, completeTradeUC: completeTradeUC}
}

// tradeIDFromURL はパスの {id} を交換のIDとして読む。UUIDとして読めなければfalseを返す。
func tradeIDFromURL(r *http.Request) (model.TradeID, bool) {
	parsed, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return model.TradeID{}, false
	}

	return model.TradeID(parsed), true
}
