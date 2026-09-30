// Package trade_approval は、交換の申し込みの承認のハンドラーを提供する。
package trade_approval

import (
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は申し込みの承認のHTTPハンドラー。
type Handler struct {
	errorRenderer  *httperror.Renderer
	flashMgr       *session.FlashManager
	approveTradeUC *usecase.ApproveTradeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(errorRenderer *httperror.Renderer, flashMgr *session.FlashManager, approveTradeUC *usecase.ApproveTradeUsecase) *Handler {
	return &Handler{errorRenderer: errorRenderer, flashMgr: flashMgr, approveTradeUC: approveTradeUC}
}
