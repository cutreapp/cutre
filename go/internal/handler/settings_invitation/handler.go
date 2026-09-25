// Package settings_invitation は招待の画面のハンドラーを提供する。
package settings_invitation

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は招待の画面のHTTPハンドラー。
type Handler struct {
	cfg                        *config.Config
	errorRenderer              *httperror.Renderer
	flashMgr                   *session.FlashManager
	prepareInvitationUC        *usecase.PrepareInvitationUsecase
	recreateInvitationUC       *usecase.RecreateInvitationUsecase
	getInvitationRedemptionsUC *usecase.GetInvitationRedemptionsUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	flashMgr *session.FlashManager,
	prepareInvitationUC *usecase.PrepareInvitationUsecase,
	recreateInvitationUC *usecase.RecreateInvitationUsecase,
	getInvitationRedemptionsUC *usecase.GetInvitationRedemptionsUsecase,
) *Handler {
	return &Handler{
		cfg:                        cfg,
		errorRenderer:              errorRenderer,
		flashMgr:                   flashMgr,
		prepareInvitationUC:        prepareInvitationUC,
		recreateInvitationUC:       recreateInvitationUC,
		getInvitationRedemptionsUC: getInvitationRedemptionsUC,
	}
}
