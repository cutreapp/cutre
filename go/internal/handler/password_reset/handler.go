// Package password_reset はパスワードリセットを申請するハンドラーを提供する。
// 申請の画面の表示 (GET /password_reset) と、申請の受け付け (POST /password_reset) を扱う。
package password_reset

import (
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/turnstile"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// レート制限の上限。申請は1回ごとにメールを送りうるため、登録の確認コードの送信と同じ枠と回数にする。
const (
	rateLimitAction = "password_reset"
	rateLimitWindow = time.Hour
	ipRateLimit     = 20
	emailRateLimit  = 5
)

// Handler はパスワードリセットを申請するHTTPハンドラー。
type Handler struct {
	cfg                   *config.Config
	limiter               *ratelimit.Limiter
	turnstile             turnstile.Verifier
	createPasswordResetUC *usecase.CreatePasswordResetUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	limiter *ratelimit.Limiter,
	turnstileVerifier turnstile.Verifier,
	createPasswordResetUC *usecase.CreatePasswordResetUsecase,
) *Handler {
	return &Handler{
		cfg:                   cfg,
		limiter:               limiter,
		turnstile:             turnstileVerifier,
		createPasswordResetUC: createPasswordResetUC,
	}
}
