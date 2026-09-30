package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetEndedTradeCountsUsecase は、マイページに出す、ユーザーの終わった交換の段階ごとの数を引く。
type GetEndedTradeCountsUsecase struct {
	tradeRepo *repository.TradeRepository
}

// NewGetEndedTradeCountsUsecase は GetEndedTradeCountsUsecase を生成する。
func NewGetEndedTradeCountsUsecase(tradeRepo *repository.TradeRepository) *GetEndedTradeCountsUsecase {
	return &GetEndedTradeCountsUsecase{tradeRepo: tradeRepo}
}

// GetEndedTradeCountsInput は GetEndedTradeCountsUsecase.Execute の入力。
type GetEndedTradeCountsInput struct {
	UserID model.UserID
}

// GetEndedTradeCountsOutput は GetEndedTradeCountsUsecase.Execute の結果。
type GetEndedTradeCountsOutput struct {
	// Counts は終わった交換の段階ごとの数。1件も無い段階は入らない。
	Counts map[model.TradeStatus]int64
}

// Execute はユーザーの終わった交換の段階ごとの数を返す。
func (uc *GetEndedTradeCountsUsecase) Execute(ctx context.Context, input GetEndedTradeCountsInput) (*GetEndedTradeCountsOutput, error) {
	counts, err := uc.tradeRepo.CountEndedByUserIDGroupByStatus(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("終わった交換の数の取得に失敗: %w", err)
	}

	return &GetEndedTradeCountsOutput{Counts: counts}, nil
}
