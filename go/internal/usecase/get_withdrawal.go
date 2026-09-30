package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetWithdrawalUsecase は、退会の画面に出す、退会できるかどうかに関わる情報を引く。
type GetWithdrawalUsecase struct {
	tradeRepo *repository.TradeRepository
}

// NewGetWithdrawalUsecase は GetWithdrawalUsecase を生成する。
func NewGetWithdrawalUsecase(tradeRepo *repository.TradeRepository) *GetWithdrawalUsecase {
	return &GetWithdrawalUsecase{tradeRepo: tradeRepo}
}

// GetWithdrawalInput は GetWithdrawalUsecase.Execute の入力。
type GetWithdrawalInput struct {
	UserID model.UserID
}

// GetWithdrawalOutput は GetWithdrawalUsecase.Execute の結果。
type GetWithdrawalOutput struct {
	// InProgressTradeCount は、ユーザーの進行中 (返事待ち・マッチ成立) の交換の数。1件以上あれば退会できない。
	InProgressTradeCount int64
}

// Execute はユーザーの進行中の交換の数を返す。
// 画面を開いたあとに交換が進んでも、退会のUseCaseがトランザクションの中で確かめ直す。
func (uc *GetWithdrawalUsecase) Execute(ctx context.Context, input GetWithdrawalInput) (*GetWithdrawalOutput, error) {
	count, err := uc.tradeRepo.CountInProgressByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("進行中の交換の数の取得に失敗: %w", err)
	}

	return &GetWithdrawalOutput{InProgressTradeCount: count}, nil
}
