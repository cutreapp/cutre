// Package trade_confirmation は、交換の申し込み内容の確認のハンドラーを提供する。
package trade_confirmation

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は申し込み内容の確認のHTTPハンドラー。
type Handler struct {
	cfg                *config.Config
	errorRenderer      *httperror.Renderer
	getTradeProposalUC *usecase.GetTradeProposalUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, errorRenderer *httperror.Renderer, getTradeProposalUC *usecase.GetTradeProposalUsecase) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getTradeProposalUC: getTradeProposalUC}
}
