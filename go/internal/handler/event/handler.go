// Package event はユーザー向けのイベントの一覧と、イベントのカテゴリーの一覧のハンドラーを提供する。
// リストに追加するグッズを、イベント → カテゴリー → グッズの順にたどる入口になる。
package event

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はユーザー向けのイベントのHTTPハンドラー。
type Handler struct {
	cfg           *config.Config
	errorRenderer *httperror.Renderer
	getEventsUC   *usecase.GetEventsUsecase
	getEventUC    *usecase.GetEventUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, errorRenderer *httperror.Renderer, getEventsUC *usecase.GetEventsUsecase, getEventUC *usecase.GetEventUsecase) *Handler {
	return &Handler{cfg: cfg, errorRenderer: errorRenderer, getEventsUC: getEventsUC, getEventUC: getEventUC}
}
