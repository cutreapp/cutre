// Package password_reset_sent はパスワードリセットの申請を受け付けた後の画面のハンドラーを提供する。
// 申請 (POST /password_reset) の後の303の行き先 (GET /password_reset/sent) を扱う。
package password_reset_sent

import "github.com/cutreapp/cutre/go/internal/config"

// Handler はパスワードリセットの申請を受け付けた後の画面のHTTPハンドラー。
type Handler struct {
	cfg *config.Config
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{cfg: cfg}
}
