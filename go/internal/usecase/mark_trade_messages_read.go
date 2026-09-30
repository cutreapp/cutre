package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// MarkTradeMessagesReadUsecase は、交換の2人のどちらかが、交換のメッセージをどこまで読んだかを記録する。
type MarkTradeMessagesReadUsecase struct {
	tradeRepo            *repository.TradeRepository
	tradeMessageReadRepo *repository.TradeMessageReadRepository
}

// NewMarkTradeMessagesReadUsecase は MarkTradeMessagesReadUsecase を生成する。
func NewMarkTradeMessagesReadUsecase(tradeRepo *repository.TradeRepository, tradeMessageReadRepo *repository.TradeMessageReadRepository) *MarkTradeMessagesReadUsecase {
	return &MarkTradeMessagesReadUsecase{tradeRepo: tradeRepo, tradeMessageReadRepo: tradeMessageReadRepo}
}

// MarkTradeMessagesReadInput は MarkTradeMessagesReadUsecase.Execute の入力。
type MarkTradeMessagesReadInput struct {
	UserID  model.UserID
	TradeID model.TradeID
	// LastReadPosition は、ユーザーに見せたメッセージの最後の位置。
	// 読んだ時刻ではなく見せたメッセージの位置にし、見せる前に届いたメッセージを読んだことにしない。
	LastReadPosition model.TradeMessageReadPosition
}

// Execute は、ユーザーが交換 TradeID のメッセージを LastReadPosition まで読んだことを記録する。
// すでにそれより後まで読んでいたときは、読んだ位置を戻さない。
//
// 交換が無いときと、ユーザーが交換の2人のどちらでもないときは AppErrCodeResourceNotFound を返す。
func (uc *MarkTradeMessagesReadUsecase) Execute(ctx context.Context, input MarkTradeMessagesReadInput) error {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return fmt.Errorf("交換の取得に失敗: %w", err)
	}
	if trade == nil || !policy.NewTradePolicy(input.UserID, trade).CanView() {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"trade_id": input.TradeID.String(), "user_id": input.UserID.String()}}
	}

	if err := uc.tradeMessageReadRepo.Save(ctx, trade.ID, input.UserID, input.LastReadPosition); err != nil {
		return fmt.Errorf("交換のメッセージをどこまで読んだかの記録に失敗: %w", err)
	}

	return nil
}
