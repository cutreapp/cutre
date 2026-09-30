// Package list はユーザー向けのリスト (譲れる・ほしい) のハンドラーを提供する。
package list

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はユーザー向けのリストのHTTPハンドラー。
type Handler struct {
	cfg       *config.Config
	getListUC *usecase.GetListUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, getListUC *usecase.GetListUsecase) *Handler {
	return &Handler{cfg: cfg, getListUC: getListUC}
}
