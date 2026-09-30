// Package match はマッチ候補のハンドラーを提供する。
package match

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はマッチ候補のHTTPハンドラー。
type Handler struct {
	cfg          *config.Config
	getMatchesUC *usecase.GetMatchesUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, getMatchesUC *usecase.GetMatchesUsecase) *Handler {
	return &Handler{cfg: cfg, getMatchesUC: getMatchesUC}
}
