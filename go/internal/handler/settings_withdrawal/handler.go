// Package settings_withdrawal は退会の画面のハンドラーを提供する。
package settings_withdrawal

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は退会の画面のHTTPハンドラー。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	limiter         *ratelimit.Limiter
	deleteAccountUC *usecase.DeleteAccountUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	deleteAccountUC *usecase.DeleteAccountUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		sessionMgr:      sessionMgr,
		flashMgr:        flashMgr,
		limiter:         limiter,
		deleteAccountUC: deleteAccountUC,
	}
}
