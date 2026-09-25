// Package settings_two_factor_auth は二要素認証の画面のハンドラーを提供する。
package settings_two_factor_auth

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は二要素認証の画面のHTTPハンドラー。
type Handler struct {
	cfg                       *config.Config
	flashMgr                  *session.FlashManager
	limiter                   *ratelimit.Limiter
	getTwoFactorAuthStatusUC  *usecase.GetTwoFactorAuthStatusUsecase
	prepareTwoFactorAuthUC    *usecase.PrepareTwoFactorAuthUsecase
	getPendingTwoFactorAuthUC *usecase.GetPendingTwoFactorAuthUsecase
	enableTwoFactorAuthUC     *usecase.EnableTwoFactorAuthUsecase
	disableTwoFactorAuthUC    *usecase.DisableTwoFactorAuthUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	getTwoFactorAuthStatusUC *usecase.GetTwoFactorAuthStatusUsecase,
	prepareTwoFactorAuthUC *usecase.PrepareTwoFactorAuthUsecase,
	getPendingTwoFactorAuthUC *usecase.GetPendingTwoFactorAuthUsecase,
	enableTwoFactorAuthUC *usecase.EnableTwoFactorAuthUsecase,
	disableTwoFactorAuthUC *usecase.DisableTwoFactorAuthUsecase,
) *Handler {
	return &Handler{
		cfg:                       cfg,
		flashMgr:                  flashMgr,
		limiter:                   limiter,
		getTwoFactorAuthStatusUC:  getTwoFactorAuthStatusUC,
		prepareTwoFactorAuthUC:    prepareTwoFactorAuthUC,
		getPendingTwoFactorAuthUC: getPendingTwoFactorAuthUC,
		enableTwoFactorAuthUC:     enableTwoFactorAuthUC,
		disableTwoFactorAuthUC:    disableTwoFactorAuthUC,
	}
}
