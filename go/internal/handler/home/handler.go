// Package home はログイン後のホームのハンドラーを提供する。
package home

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はログイン後のホームのHTTPハンドラー。
type Handler struct {
	cfg       *config.Config
	getHomeUC *usecase.GetHomeUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, getHomeUC *usecase.GetHomeUsecase) *Handler {
	return &Handler{cfg: cfg, getHomeUC: getHomeUC}
}
