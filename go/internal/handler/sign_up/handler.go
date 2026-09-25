// Package sign_up は登録を始めるハンドラーを提供する。
// メールアドレスの入力画面の表示 (GET /sign_up) と、確認コードの送信 (POST /sign_up) を扱う。
package sign_up

import (
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/turnstile"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// レート制限の上限。確認コードの送信は1回ごとにメールを送るため、ログインより厳しくする。
//
// メールアドレスの上限は、第三者が同じアドレスへメールを送り付け続けることを抑える。
// IPアドレスの上限は、回線 (IPv6の /64、NAT) を分け合う利用者を巻き込まないよう緩くする。
const (
	rateLimitAction = "sign_up"
	rateLimitWindow = time.Hour
	ipRateLimit     = 20
	emailRateLimit  = 5
)

// Handler は登録を始めるHTTPハンドラー。
type Handler struct {
	cfg                 *config.Config
	continuationMgr     *session.ContinuationManager
	limiter             *ratelimit.Limiter
	turnstile           turnstile.Verifier
	getInvitationByIDUC *usecase.GetInvitationByIDUsecase
	createSignUpUC      *usecase.CreateSignUpUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	limiter *ratelimit.Limiter,
	turnstileVerifier turnstile.Verifier,
	getInvitationByIDUC *usecase.GetInvitationByIDUsecase,
	createSignUpUC *usecase.CreateSignUpUsecase,
) *Handler {
	return &Handler{
		cfg:                 cfg,
		continuationMgr:     continuationMgr,
		limiter:             limiter,
		turnstile:           turnstileVerifier,
		getInvitationByIDUC: getInvitationByIDUC,
		createSignUpUC:      createSignUpUC,
	}
}
