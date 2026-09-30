// Package trade_message は交換のメッセージのハンドラーを提供する。
// 交換のメッセージのページ (GET) と、メッセージの送信 (POST) を扱う。取り消しは trade_message_retraction が扱う。
package trade_message

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

// Handler は交換のメッセージのHTTPハンドラー。
type Handler struct {
	cfg                     *config.Config
	errorRenderer           *httperror.Renderer
	flashMgr                *session.FlashManager
	limiter                 *ratelimit.Limiter
	getTradeMessagesUC      *usecase.GetTradeMessagesUsecase
	markTradeMessagesReadUC *usecase.MarkTradeMessagesReadUsecase
	createTradeMessageUC    *usecase.CreateTradeMessageUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	getTradeMessagesUC *usecase.GetTradeMessagesUsecase,
	markTradeMessagesReadUC *usecase.MarkTradeMessagesReadUsecase,
	createTradeMessageUC *usecase.CreateTradeMessageUsecase,
) *Handler {
	return &Handler{
		cfg:                     cfg,
		errorRenderer:           errorRenderer,
		flashMgr:                flashMgr,
		limiter:                 limiter,
		getTradeMessagesUC:      getTradeMessagesUC,
		markTradeMessagesReadUC: markTradeMessagesReadUC,
		createTradeMessageUC:    createTradeMessageUC,
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
