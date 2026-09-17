// Package welcome はトップページのハンドラーを提供する。
package welcome

import "github.com/cutreapp/cutre/go/internal/config"

// Handler はトップページのHTTPハンドラー。
type Handler struct {
	cfg *config.Config
}

// NewHandler は新しいHandlerを作成する。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
