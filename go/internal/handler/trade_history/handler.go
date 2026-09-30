// Package trade_history はこれまでの交換 (終わった交換の一覧) のハンドラーを提供する。
package trade_history

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はこれまでの交換のHTTPハンドラー。
type Handler struct {
	cfg               *config.Config
	getTradeHistoryUC *usecase.GetTradeHistoryUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, getTradeHistoryUC *usecase.GetTradeHistoryUsecase) *Handler {
	return &Handler{cfg: cfg, getTradeHistoryUC: getTradeHistoryUC}
}
