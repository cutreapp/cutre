// Package profile はプロフィールのハンドラーを提供する。
package profile

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/httperror"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はプロフィールのHTTPハンドラー。
type Handler struct {
	cfg                        *config.Config
	errorRenderer              *httperror.Renderer
	getInvitationRedemptionsUC *usecase.GetInvitationRedemptionsUsecase
	getTwoFactorAuthStatusUC   *usecase.GetTwoFactorAuthStatusUsecase
	getMessageConsentUC        *usecase.GetMessageConsentUsecase
	getPlacesUC                *usecase.GetPlacesUsecase
	getProfileUC               *usecase.GetProfileUsecase
	getEndedTradeCountsUC      *usecase.GetEndedTradeCountsUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	errorRenderer *httperror.Renderer,
	getInvitationRedemptionsUC *usecase.GetInvitationRedemptionsUsecase,
	getTwoFactorAuthStatusUC *usecase.GetTwoFactorAuthStatusUsecase,
	getMessageConsentUC *usecase.GetMessageConsentUsecase,
	getPlacesUC *usecase.GetPlacesUsecase,
	getProfileUC *usecase.GetProfileUsecase,
	getEndedTradeCountsUC *usecase.GetEndedTradeCountsUsecase,
) *Handler {
	return &Handler{
		cfg:                        cfg,
		errorRenderer:              errorRenderer,
		getInvitationRedemptionsUC: getInvitationRedemptionsUC,
		getTwoFactorAuthStatusUC:   getTwoFactorAuthStatusUC,
		getMessageConsentUC:        getMessageConsentUC,
		getPlacesUC:                getPlacesUC,
		getProfileUC:               getProfileUC,
		getEndedTradeCountsUC:      getEndedTradeCountsUC,
	}
}
