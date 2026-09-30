package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetMainNavUsecase は、ログイン後のページのメインメニューに出す、項目ごとの数字を引く。
type GetMainNavUsecase struct {
	tradeRepo        *repository.TradeRepository
	tradeMessageRepo *repository.TradeMessageRepository
}

// NewGetMainNavUsecase は GetMainNavUsecase を生成する。
func NewGetMainNavUsecase(tradeRepo *repository.TradeRepository, tradeMessageRepo *repository.TradeMessageRepository) *GetMainNavUsecase {
	return &GetMainNavUsecase{tradeRepo: tradeRepo, tradeMessageRepo: tradeMessageRepo}
}

// GetMainNavInput は GetMainNavUsecase.Execute の入力。
type GetMainNavInput struct {
	UserID model.UserID
}

// GetMainNavOutput は GetMainNavUsecase.Execute の結果。
type GetMainNavOutput struct {
	// AwaitingTradeCount は、ユーザーの返事や確認を待っている交換の数。交換の項目に出す。
	AwaitingTradeCount int64
	// UnreadMessageCount は、ユーザーの交換すべての未読のメッセージの数。メッセージの項目に出す。
	UnreadMessageCount int64
}

// Execute はユーザーのメインメニューに出す数字を返す。ログイン後のページを開くたびに呼ぶため、数えるクエリだけを発行する。
func (uc *GetMainNavUsecase) Execute(ctx context.Context, input GetMainNavInput) (*GetMainNavOutput, error) {
	awaiting, err := uc.tradeRepo.CountAwaitingByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("返事や確認を待っている交換の数の取得に失敗: %w", err)
	}

	unread, err := uc.tradeMessageRepo.CountUnreadByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("未読のメッセージの数の取得に失敗: %w", err)
	}

	return &GetMainNavOutput{AwaitingTradeCount: awaiting, UnreadMessageCount: unread}, nil
}
