package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetMessageConsentUsecase は、ユーザーのメッセージの取り扱いへの最新の同意を引く。
type GetMessageConsentUsecase struct {
	messageConsentRepo *repository.MessageConsentRepository
}

// NewGetMessageConsentUsecase は GetMessageConsentUsecase を生成する。
func NewGetMessageConsentUsecase(messageConsentRepo *repository.MessageConsentRepository) *GetMessageConsentUsecase {
	return &GetMessageConsentUsecase{messageConsentRepo: messageConsentRepo}
}

// GetMessageConsentInput は GetMessageConsentUsecase.Execute の入力。
type GetMessageConsentInput struct {
	UserID model.UserID
}

// GetMessageConsentOutput は GetMessageConsentUsecase.Execute の結果。
type GetMessageConsentOutput struct {
	// Consent は最新の同意の記録。一度も同意していないときはnil。
	// やめた記録や古い版の記録のこともあるため、有効かどうかは IsValid で確かめる。
	Consent *model.MessageConsent
}

// Execute はユーザーの最新の同意の記録を返す。
func (uc *GetMessageConsentUsecase) Execute(ctx context.Context, input GetMessageConsentInput) (*GetMessageConsentOutput, error) {
	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}

	return &GetMessageConsentOutput{Consent: consent}, nil
}
