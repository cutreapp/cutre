// Package trade_message_retraction は、交換のメッセージの取り消しのハンドラーを提供する。
package trade_message_retraction

import (
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はメッセージの取り消しのHTTPハンドラー。
type Handler struct {
	errorRenderer         *httperror.Renderer
	flashMgr              *session.FlashManager
	retractTradeMessageUC *usecase.RetractTradeMessageUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(errorRenderer *httperror.Renderer, flashMgr *session.FlashManager, retractTradeMessageUC *usecase.RetractTradeMessageUsecase) *Handler {
	return &Handler{errorRenderer: errorRenderer, flashMgr: flashMgr, retractTradeMessageUC: retractTradeMessageUC}
}
