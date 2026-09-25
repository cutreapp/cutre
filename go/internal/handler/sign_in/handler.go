// Package sign_in はログインのハンドラーを提供する。
// ログイン画面の表示 (GET /sign_in) と、メールアドレスとパスワードの照合からセッションの発行まで (POST /sign_in) を扱う。
// 二要素認証を有効にしたユーザーは、セッションを発行せずに認証アプリのコードの入力 (sign_in_two_factor) へ送る。
package sign_in

import (
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/turnstile"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// レート制限の上限。どちらも固定ウィンドウで、成功したログインも1回と数える。
//
// IPアドレスの上限はメールアドレスより緩くする。同じ回線 (IPv6の /64、家庭や学校のNAT) を
// 複数の利用者が分け合うことがあり、1人の打ち間違いで他の人がログインできなくならないようにするため。
// メールアドレスの上限は、1つのアカウントへのパスワードの総当たりを抑える。
// これは第三者がその利用者のログインを枠の終わりまで止められることでもあるため、枠は15分と短くする。
const (
	rateLimitAction = "sign_in"
	rateLimitWindow = 15 * time.Minute
	ipRateLimit     = 30
	emailRateLimit  = 10
)

// Handler はログインのHTTPハンドラー。
type Handler struct {
	cfg             *config.Config
	sessionMgr      *session.Manager
	continuationMgr *session.ContinuationManager
	flashMgr        *session.FlashManager
	limiter         *ratelimit.Limiter
	turnstile       turnstile.Verifier
	createSignInUC  *usecase.CreateSignInUsecase
	createSessionUC *usecase.CreateSessionUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	sessionMgr *session.Manager,
	continuationMgr *session.ContinuationManager,
	flashMgr *session.FlashManager,
	limiter *ratelimit.Limiter,
	turnstileVerifier turnstile.Verifier,
	createSignInUC *usecase.CreateSignInUsecase,
	createSessionUC *usecase.CreateSessionUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		sessionMgr:      sessionMgr,
		continuationMgr: continuationMgr,
		flashMgr:        flashMgr,
		limiter:         limiter,
		turnstile:       turnstileVerifier,
		createSignInUC:  createSignInUC,
		createSessionUC: createSessionUC,
	}
}
