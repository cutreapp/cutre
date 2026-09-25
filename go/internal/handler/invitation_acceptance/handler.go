// Package invitation_acceptance は招待リンクの受け取りのハンドラーを提供する。
// 招待の確認画面の表示 (GET /i/{token}) と、受け取った招待を持って登録を始める操作 (POST /i/{token}) を扱う。
package invitation_acceptance

import (
	"time"

	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/ratelimit"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// レート制限の上限。表示と登録の開始をまとめて、IPアドレスの単位で数える。
//
// トークンは推測できない長さの乱数で、総当たりで招待を見つけることはもともと現実的でない。
// それでも、招待が使えるかどうかを大量に問い合わせる経路を塞ぐため、画面の利用には十分な回数で抑える。
const (
	rateLimitAction = "invitation"
	rateLimitWindow = 15 * time.Minute
	ipRateLimit     = 30
)

// Handler は招待リンクの受け取りのHTTPハンドラー。
type Handler struct {
	cfg             *config.Config
	continuationMgr *session.ContinuationManager
	limiter         *ratelimit.Limiter
	getInvitationUC *usecase.GetInvitationUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	continuationMgr *session.ContinuationManager,
	limiter *ratelimit.Limiter,
	getInvitationUC *usecase.GetInvitationUsecase,
) *Handler {
	return &Handler{
		cfg:             cfg,
		continuationMgr: continuationMgr,
		limiter:         limiter,
		getInvitationUC: getInvitationUC,
	}
}
