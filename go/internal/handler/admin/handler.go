// Package admin は管理画面の入口のハンドラーを提供する。
package admin

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は管理画面の入口のHTTPハンドラー。
type Handler struct {
	cfg            *config.Config
	errorRenderer  *httperror.Renderer
	getAdminMenuUC *usecase.GetAdminMenuUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, errorRenderer *httperror.Renderer, getAdminMenuUC *usecase.GetAdminMenuUsecase) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getAdminMenuUC: getAdminMenuUC}
}
