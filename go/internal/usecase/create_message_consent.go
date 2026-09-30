package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// CreateMessageConsentUsecase は、ユーザーがメッセージの取り扱いの今の文面に同意したことを記録する。
// 同意をやめた人と、古い版の文面に同意していた人が、もう一度同意するときに使う。
type CreateMessageConsentUsecase struct {
	messageConsentRepo *repository.MessageConsentRepository
}

// NewCreateMessageConsentUsecase は CreateMessageConsentUsecase を生成する。
func NewCreateMessageConsentUsecase(messageConsentRepo *repository.MessageConsentRepository) *CreateMessageConsentUsecase {
	return &CreateMessageConsentUsecase{messageConsentRepo: messageConsentRepo}
}

// CreateMessageConsentInput は CreateMessageConsentUsecase.Execute の入力。
type CreateMessageConsentInput struct {
	UserID model.UserID
}

// Execute は今の版の文面への同意を記録する。
//
// 既に有効な同意があるとき (二重送信や別の画面で先に同意した) は記録を増やさず、
// AppErrCodeConflict の *model.AppError を返す。
// 確かめてから記録するまでの間に別の送信が同意しても、有効な記録が2つになるだけで、どちらも同じ版の同意のため害は無い。
func (uc *CreateMessageConsentUsecase) Execute(ctx context.Context, input CreateMessageConsentInput) error {
	latest, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}
	if latest != nil && latest.IsValid() {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}

	if _, err := uc.messageConsentRepo.Create(ctx, input.UserID, model.CurrentMessageConsentVersion); err != nil {
		return fmt.Errorf("メッセージの取り扱いへの同意の記録に失敗: %w", err)
	}

	return nil
}
