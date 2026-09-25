// Package home はログイン後のホームのハンドラーを提供する。
package home

import "github.com/cutreapp/cutre/go/internal/config"

// Handler はログイン後のホームのHTTPハンドラー。
type Handler struct {
	cfg *config.Config
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
