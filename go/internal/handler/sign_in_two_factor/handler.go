// Package sign_in_two_factor は、ログインで認証アプリのコードを入力する画面のハンドラーを提供する。
// パスワードを確かめた二要素認証のユーザーに、画面の表示 (GET /sign_in/two_factor) と、
// コードの照合からセッションの発行まで (POST /sign_in/two_factor) を扱う。
package sign_in_two_factor

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は、ログインで認証アプリのコードを入力する画面のHTTPハンドラー。
type Handler struct {
	cfg                     *config.Config
	continuationMgr         *session.ContinuationManager
	sessionMgr              *session.Manager
	flashMgr                *session.FlashManager
	limiter                 *ratelimit.Limiter
	createSignInTwoFactorUC *usecase.CreateSignInTwoFactorUsecase
	createSessionUC         *usecase.CreateSessionUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	createSignInTwoFactorUC *usecase.CreateSignInTwoFactorUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
) *Handler {
	return &Handler{
		cfg:                     cfg,
		continuationMgr:         continuationMgr,
		sessionMgr:              sessionMgr,
		flashMgr:                flashMgr,
		limiter:                 limiter,
		createSignInTwoFactorUC: createSignInTwoFactorUC,
		createSessionUC:         createSessionUC,
	}
}
