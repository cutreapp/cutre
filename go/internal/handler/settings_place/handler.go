// Package settings_place は交換場所の画面のハンドラーを提供する。
// 選んだ駅と「ほかに出られるところ」の表示 (GET) と保存 (PATCH) を扱う。
package settings_place

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は交換場所の画面のHTTPハンドラー。
type Handler struct {
	cfg                    *config.Config
	errorRenderer          *httperror.Renderer
	flashMgr               *session.FlashManager
	getPlacesUC            *usecase.GetPlacesUsecase
	getPublishedStationsUC *usecase.GetPublishedStationsUsecase
	updatePlacesUC         *usecase.UpdatePlacesUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	getPlacesUC *usecase.GetPlacesUsecase,
	getPublishedStationsUC *usecase.GetPublishedStationsUsecase,
	updatePlacesUC *usecase.UpdatePlacesUsecase,
) *Handler {
	return &Handler{
		cfg:                    cfg,
		errorRenderer:          errorRenderer,
		flashMgr:               flashMgr,
		getPlacesUC:            getPlacesUC,
		getPublishedStationsUC: getPublishedStationsUC,
		updatePlacesUC:         updatePlacesUC,
	}
}
