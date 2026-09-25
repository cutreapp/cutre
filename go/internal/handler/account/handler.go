// Package account はアカウントの作成のハンドラーを提供する。
// アットネームとパスワードの入力画面の表示 (GET /account) と、アカウントの作成からログインまで (POST /account) を扱う。
package account

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はアカウントの作成のHTTPハンドラー。
type Handler struct {
	cfg                             *config.Config
	continuationMgr                 *session.ContinuationManager
	sessionMgr                      *session.Manager
	flashMgr                        *session.FlashManager
	getInvitationByIDUC             *usecase.GetInvitationByIDUsecase
	getConfirmedEmailConfirmationUC *usecase.GetConfirmedEmailConfirmationUsecase
	createAccountUC                 *usecase.CreateAccountUsecase
	createSessionUC                 *usecase.CreateSessionUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	getInvitationByIDUC *usecase.GetInvitationByIDUsecase,
	getConfirmedEmailConfirmationUC *usecase.GetConfirmedEmailConfirmationUsecase,
	createAccountUC *usecase.CreateAccountUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
) *Handler {
	return &Handler{
		cfg:                             cfg,
		continuationMgr:                 continuationMgr,
		sessionMgr:                      sessionMgr,
		flashMgr:                        flashMgr,
		getInvitationByIDUC:             getInvitationByIDUC,
		getConfirmedEmailConfirmationUC: getConfirmedEmailConfirmationUC,
		createAccountUC:                 createAccountUC,
		createSessionUC:                 createSessionUC,
	}
}
