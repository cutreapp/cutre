// Package email_confirmation はメールアドレスの確認コードの入力画面を提供する。
// 画面の表示 (GET)、コードの照合 (POST)、コードの再送 (PATCH) を扱う。
package email_confirmation

import (
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// コードの照合のレート制限。1つの確認コードへの誤入力は確認ごとに上限があるため、
// ここでは確認を作り直しながら続ける総当たりを、IPアドレスの単位で抑える。
const (
	verifyRateLimitAction = "email_confirmation"
	verifyRateLimitWindow = 15 * time.Minute
	verifyIPRateLimit     = 30
)

// コードの再送のレート制限。再送も1回ごとにメールを送るため、登録の開始 (sign_up) と同じ枠と回数にする。
const (
	resendRateLimitAction = "email_confirmation_resend"
	resendRateLimitWindow = time.Hour
	resendIPRateLimit     = 20
	resendEmailRateLimit  = 5
)

// Handler は確認コードの入力画面のHTTPハンドラー。
type Handler struct {
	cfg                       *config.Config
	continuationMgr           *session.ContinuationManager
	flashMgr                  *session.FlashManager
	limiter                   *ratelimit.Limiter
	getInvitationByIDUC       *usecase.GetInvitationByIDUsecase
	getEmailConfirmationUC    *usecase.GetEmailConfirmationUsecase
	verifyEmailConfirmationUC *usecase.VerifyEmailConfirmationUsecase
	createSignUpUC            *usecase.CreateSignUpUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	getInvitationByIDUC *usecase.GetInvitationByIDUsecase,
	getEmailConfirmationUC *usecase.GetEmailConfirmationUsecase,
	verifyEmailConfirmationUC *usecase.VerifyEmailConfirmationUsecase,
	createSignUpUC *usecase.CreateSignUpUsecase,
) *Handler {
	return &Handler{
		cfg:                       cfg,
		continuationMgr:           continuationMgr,
		flashMgr:                  flashMgr,
		limiter:                   limiter,
		getInvitationByIDUC:       getInvitationByIDUC,
		getEmailConfirmationUC:    getEmailConfirmationUC,
		verifyEmailConfirmationUC: verifyEmailConfirmationUC,
		createSignUpUC:            createSignUpUC,
	}
}
