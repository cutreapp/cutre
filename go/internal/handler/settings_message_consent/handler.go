// Package settings_message_consent はメッセージの利用 (メッセージの取り扱いへの同意) の画面のハンドラーを提供する。
// 同意した日と内容の表示 (GET)、同意 (POST)、同意をやめる (DELETE) を扱う。
package settings_message_consent

import (
	"github.com/cutreapp/cutre/go/internal/config"
	"github.com/cutreapp/cutre/go/internal/session"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// Handler はメッセージの利用の画面のHTTPハンドラー。
type Handler struct {
	cfg                      *config.Config
	flashMgr                 *session.FlashManager
	getMessageConsentUC      *usecase.GetMessageConsentUsecase
	createMessageConsentUC   *usecase.CreateMessageConsentUsecase
	withdrawMessageConsentUC *usecase.WithdrawMessageConsentUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	cfg *config.Config,
	flashMgr *session.FlashManager,
	getMessageConsentUC *usecase.GetMessageConsentUsecase,
	createMessageConsentUC *usecase.CreateMessageConsentUsecase,
	withdrawMessageConsentUC *usecase.WithdrawMessageConsentUsecase,
) *Handler {
	return &Handler{
		cfg:                      cfg,
		flashMgr:                 flashMgr,
		getMessageConsentUC:      getMessageConsentUC,
		createMessageConsentUC:   createMessageConsentUC,
		withdrawMessageConsentUC: withdrawMessageConsentUC,
	}
}
