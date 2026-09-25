// Package user_session はログイン中のセッションのハンドラーを提供する。
// セッションの削除 (DELETE /user_session) でログアウトさせる。
package user_session

import (
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はセッションのHTTPハンドラー。
type Handler struct {
	sessionMgr      *session.Manager
	flashMgr        *session.FlashManager
	deleteSessionUC *usecase.DeleteSessionUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	deleteSessionUC *usecase.DeleteSessionUsecase,
) *Handler {
	return &Handler{
		sessionMgr:      sessionMgr,
		flashMgr:        flashMgr,
		deleteSessionUC: deleteSessionUC,
	}
}
