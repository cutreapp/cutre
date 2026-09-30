package validator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// MessageConsentWithdrawValidator は、メッセージの取り扱いへの同意をやめられるかを検証する。
type MessageConsentWithdrawValidator struct {
	tradeRepo *repository.TradeRepository
}

// NewMessageConsentWithdrawValidator は MessageConsentWithdrawValidator を生成する。
func NewMessageConsentWithdrawValidator(tradeRepo *repository.TradeRepository) *MessageConsentWithdrawValidator {
	return &MessageConsentWithdrawValidator{tradeRepo: tradeRepo}
}

// WithTx はtx内で交換を確認する新しいValidatorを返す。
func (v *MessageConsentWithdrawValidator) WithTx(tx *sql.Tx) *MessageConsentWithdrawValidator {
	return &MessageConsentWithdrawValidator{tradeRepo: v.tradeRepo.WithTx(tx)}
}

// Validate は、ユーザー userID に進行中 (返事待ち・マッチ成立) の交換が無いかを確かめる。
// あるときは、交換が終わるまで同意をやめられないことを伝える *model.ValidationError を返す。
// 進行中の交換ではメッセージでやり取りしているため、その途中で同意をやめさせない。
func (v *MessageConsentWithdrawValidator) Validate(ctx context.Context, userID model.UserID) error {
	inProgress, err := v.tradeRepo.ExistsInProgressByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("進行中の交換の確認に失敗: %w", err)
	}
	if inProgress {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "settings_message_consent_withdraw_trade_in_progress"))
		return ve
	}

	return nil
}
