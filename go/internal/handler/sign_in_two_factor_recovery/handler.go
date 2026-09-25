// Package sign_in_two_factor_recovery は、ログインでリカバリーコードを入力する画面のハンドラーを提供する。
// パスワードを確かめた二要素認証のユーザーのうち、認証アプリを使えない人に、画面の表示 (GET /sign_in/two_factor/recovery) と、
// コードの消費からセッションの発行まで (POST /sign_in/two_factor/recovery) を扱う。
package sign_in_two_factor_recovery

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler は、ログインでリカバリーコードを入力する画面のHTTPハンドラー。
type Handler struct {
	cfg                             *config.Config
	continuationMgr                 *session.ContinuationManager
	sessionMgr                      *session.Manager
	flashMgr                        *session.FlashManager
	limiter                         *ratelimit.Limiter
	createSignInTwoFactorRecoveryUC *usecase.CreateSignInTwoFactorRecoveryUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	sessionMgr *session.Manager,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	createSignInTwoFactorRecoveryUC *usecase.CreateSignInTwoFactorRecoveryUsecase,
) *Handler {
	return &Handler{
		cfg:                             cfg,
		continuationMgr:                 continuationMgr,
		sessionMgr:                      sessionMgr,
		flashMgr:                        flashMgr,
		limiter:                         limiter,
		createSignInTwoFactorRecoveryUC: createSignInTwoFactorRecoveryUC,
	}
}
