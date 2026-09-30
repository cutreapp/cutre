// Package trade_withdrawal は、交換の申し込みの取り下げのハンドラーを提供する。
package trade_withdrawal

import (
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は申し込みの取り下げのHTTPハンドラー。
type Handler struct {
	errorRenderer   *httperror.Renderer
	flashMgr        *session.FlashManager
	withdrawTradeUC *usecase.WithdrawTradeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(errorRenderer *httperror.Renderer, flashMgr *session.FlashManager, withdrawTradeUC *usecase.WithdrawTradeUsecase) *Handler {
	return &Handler{errorRenderer: errorRenderer, flashMgr: flashMgr, withdrawTradeUC: withdrawTradeUC}
}
