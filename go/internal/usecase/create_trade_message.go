package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateTradeMessageUsecase は、交換の2人のどちらかが、進行中の交換でメッセージを送る。
type CreateTradeMessageUsecase struct {
	validator          *validator.TradeMessageCreateValidator
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeMessageRepo   *repository.TradeMessageRepository
}

// NewCreateTradeMessageUsecase は CreateTradeMessageUsecase を生成する。
func NewCreateTradeMessageUsecase(
	validator *validator.TradeMessageCreateValidator,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
) *CreateTradeMessageUsecase {
	return &CreateTradeMessageUsecase{
		validator:          validator,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeMessageRepo:   tradeMessageRepo,
	}
}

// CreateTradeMessageInput は CreateTradeMessageUsecase.Execute の入力。本文はフォームの値をそのまま受け取る。
type CreateTradeMessageInput struct {
	SenderUserID model.UserID
	TradeID      model.TradeID
	Body         string
}

// CreateTradeMessageOutput は CreateTradeMessageUsecase.Execute の結果。
type CreateTradeMessageOutput struct {
	Message *model.TradeMessage
}

// Execute は交換 TradeID でメッセージを送る。
//
// 交換が無いときと、送る人が交換の2人のどちらでもないときは AppErrCodeResourceNotFound を返す。
// 送る人にメッセージの取り扱いへの有効な同意が無いときは AppErrCodeMessageConsentRequired を返す。
// 交換が終わっていた (画面を開いたあとに終わった) ときは、送らずに AppErrCodeConflict を返す。
// 本文の誤りは *model.ValidationError で返す。
//
// 進行中の交換の2人は同意をやめられないため、同意を確かめてから送るまでの間に同意が無くなることはない。
// 交換が終わる操作とは、交換の段階を条件にした記録で直列化する。
func (uc *CreateTradeMessageUsecase) Execute(ctx context.Context, input CreateTradeMessageInput) (*CreateTradeMessageOutput, error) {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return nil, fmt.Errorf("交換の取得に失敗: %w", err)
	}
	metadata := map[string]string{"trade_id": input.TradeID.String(), "user_id": input.SenderUserID.String()}
	if trade == nil || !policy.NewTradePolicy(input.SenderUserID, trade).CanView() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: metadata}
	}
	if !trade.IsInProgress() {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.SenderUserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}
	if consent == nil || !consent.IsValid() {
		return nil, &model.AppError{Code: model.AppErrCodeMessageConsentRequired, Metadata: metadata}
	}

	attrs, err := uc.validator.Validate(ctx, validator.TradeMessageCreateValidatorInput{Body: input.Body})
	if err != nil {
		return nil, err
	}

	message, err := uc.tradeMessageRepo.CreateInProgress(ctx, trade.ID, input.SenderUserID, attrs.Body)
	if err != nil {
		return nil, fmt.Errorf("交換のメッセージの記録に失敗: %w", err)
	}
	if message == nil {
		return nil, &model.AppError{Code: model.AppErrCodeConflict, Metadata: metadata}
	}

	slog.InfoContext(ctx, "交換のメッセージを送りました", "trade_id", trade.ID, "trade_message_id", message.ID, "sender_user_id", input.SenderUserID)

	return &CreateTradeMessageOutput{Message: message}, nil
}
