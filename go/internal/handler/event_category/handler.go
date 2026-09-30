// Package event_category はユーザー向けのカテゴリーのグッズの一覧のハンドラーを提供する。
// グッズごとに、自分のリストに入れているかと、リストに追加する画面への入口を出す。
package event_category

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はユーザー向けのカテゴリーのHTTPハンドラー。
type Handler struct {
	cfg                *config.Config
	errorRenderer      *httperror.Renderer
	getEventCategoryUC *usecase.GetEventCategoryUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, errorRenderer *httperror.Renderer, getEventCategoryUC *usecase.GetEventCategoryUsecase) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getEventCategoryUC: getEventCategoryUC}
}
