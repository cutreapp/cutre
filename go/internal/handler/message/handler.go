// Package message はメッセージの一覧のハンドラーを提供する。
// 交換ごとのメッセージのページは trade_message が扱う。
package message

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はメッセージの一覧のHTTPハンドラー。
type Handler struct {
	cfg           *config.Config
	getMessagesUC *usecase.GetMessagesUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(cfg *config.Config, getMessagesUC *usecase.GetMessagesUsecase) *Handler {
	return &Handler{cfg: cfg, getMessagesUC: getMessagesUC}
}
