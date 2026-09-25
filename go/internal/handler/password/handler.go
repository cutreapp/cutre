// Package password はパスワードリセットで新しいパスワードを設定するハンドラーを提供する。
// メールのリンクから開く設定画面の表示 (GET /password) と、新しいパスワードの設定 (PATCH /password) を扱う。
// リンクの申請は password_reset が扱う。
package password

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は新しいパスワードの設定のHTTPハンドラー。
type Handler struct {
	cfg                         *config.Config
	continuationMgr             *session.ContinuationManager
	flashMgr                    *session.FlashManager
	getPasswordResetTokenUC     *usecase.GetPasswordResetTokenUsecase
	getPasswordResetTokenByIDUC *usecase.GetPasswordResetTokenByIDUsecase
	updatePasswordUC            *usecase.UpdatePasswordUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	flashMgr *session.FlashManager,
	getPasswordResetTokenUC *usecase.GetPasswordResetTokenUsecase,
	getPasswordResetTokenByIDUC *usecase.GetPasswordResetTokenByIDUsecase,
	updatePasswordUC *usecase.UpdatePasswordUsecase,
) *Handler {
	return &Handler{
		cfg:                         cfg,
		continuationMgr:             continuationMgr,
		flashMgr:                    flashMgr,
		getPasswordResetTokenUC:     getPasswordResetTokenUC,
		getPasswordResetTokenByIDUC: getPasswordResetTokenByIDUC,
		updatePasswordUC:            updatePasswordUC,
	}
}
