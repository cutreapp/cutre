// Package trade は交換のハンドラーを提供する。
// 交換の画面 (進行中の交換の一覧)・交換のページと、交換を申し込む組み合わせを選ぶ画面 (GET)・申し込み (POST) を扱う。
package trade

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は交換のHTTPハンドラー。
type Handler struct {
	cfg                *config.Config
	errorRenderer      *httperror.Renderer
	flashMgr           *session.FlashManager
	limiter            *ratelimit.Limiter
	getTradesUC        *usecase.GetTradesUsecase
	getMatchesUC       *usecase.GetMatchesUsecase
	getTradeUC         *usecase.GetTradeUsecase
	getTradeProposalUC *usecase.GetTradeProposalUsecase
	createTradeUC      *usecase.CreateTradeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	getTradesUC *usecase.GetTradesUsecase,
	getMatchesUC *usecase.GetMatchesUsecase,
	getTradeUC *usecase.GetTradeUsecase,
	getTradeProposalUC *usecase.GetTradeProposalUsecase,
	createTradeUC *usecase.CreateTradeUsecase,
) *Handler {
	return &Handler{
		cfg:                cfg,
		errorRenderer:      errorRenderer,
		flashMgr:           flashMgr,
		limiter:            limiter,
		getTradesUC:        getTradesUC,
		getMatchesUC:       getMatchesUC,
		getTradeUC:         getTradeUC,
		getTradeProposalUC: getTradeProposalUC,
		createTradeUC:      createTradeUC,
	}
}

// tradeIDFromURL はパスの {id} を交換のIDとして読む。UUIDとして読めなければfalseを返す。
func tradeIDFromURL(r *http.Request) (model.TradeID, bool) {
	parsed, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return model.TradeID{}, false
	}

	return model.TradeID(parsed), true
}
