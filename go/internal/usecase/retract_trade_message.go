package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// RetractTradeMessageUsecase は、交換のメッセージを送った人が、そのメッセージを取り消す。
// 取り消しても本文は消さずに残す。利用者から問題の報告を受けたときに、運営が確認できるようにするため。
type RetractTradeMessageUsecase struct {
	tradeRepo        *repository.TradeRepository
	tradeMessageRepo *repository.TradeMessageRepository
}

// NewRetractTradeMessageUsecase は RetractTradeMessageUsecase を生成する。
func NewRetractTradeMessageUsecase(tradeRepo *repository.TradeRepository, tradeMessageRepo *repository.TradeMessageRepository) *RetractTradeMessageUsecase {
	return &RetractTradeMessageUsecase{tradeRepo: tradeRepo, tradeMessageRepo: tradeMessageRepo}
}

// RetractTradeMessageInput は RetractTradeMessageUsecase.Execute の入力。
type RetractTradeMessageInput struct {
	UserID    model.UserID
	TradeID   model.TradeID
	MessageID model.TradeMessageID
}

// Execute は交換 TradeID のメッセージ MessageID を取り消す。
// 送ったメッセージはいつでも取り消せるため、交換が終わったあとも取り消す。
// すでに取り消していた (別のタブで先に取り消した) ときは、何もせずに取り消せたものとして扱う。
//
// 交換かメッセージが無いとき、ユーザーが交換の2人のどちらでもないとき、メッセージがその交換のものでないときは AppErrCodeResourceNotFound を、
// 相手が送ったメッセージのときは AppErrCodeForbidden の *model.AppError を返す。
func (uc *RetractTradeMessageUsecase) Execute(ctx context.Context, input RetractTradeMessageInput) error {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return fmt.Errorf("交換の取得に失敗: %w", err)
	}
	message, err := uc.tradeMessageRepo.FindByID(ctx, input.MessageID)
	if err != nil {
		return fmt.Errorf("交換のメッセージの取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "trade_message_id": input.MessageID.String(), "user_id": input.UserID.String()}
	if trade == nil || message == nil || message.TradeID != trade.ID {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	tradePolicy := policy.NewTradePolicy(input.UserID, trade)
	if !tradePolicy.CanView() {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	if !tradePolicy.CanRetractMessage(message) {
		return &model.AppError{Code: model.AppErrCodeForbidden, Metadata: metadata}
	}

	retracted, err := uc.tradeMessageRepo.Retract(ctx, message.ID, input.UserID)
	if err != nil {
		return fmt.Errorf("交換のメッセージの取り消しに失敗: %w", err)
	}
	if retracted {
		slog.InfoContext(ctx, "交換のメッセージを取り消しました", "trade_id", trade.ID, "trade_message_id", message.ID, "sender_user_id", input.UserID)
	}

	return nil
}
