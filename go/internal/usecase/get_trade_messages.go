package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTradeMessagesUsecase は交換のメッセージのページに出す、交換とその相手・交換の品・メッセージ・出来事を引く。
type GetTradeMessagesUsecase struct {
	itemRepo             *repository.ItemRepository
	messageConsentRepo   *repository.MessageConsentRepository
	tradeRepo            *repository.TradeRepository
	tradeEventRepo       *repository.TradeEventRepository
	tradeItemRepo        *repository.TradeItemRepository
	tradeMessageRepo     *repository.TradeMessageRepository
	tradeMessageReadRepo *repository.TradeMessageReadRepository
	userRepo             *repository.UserRepository
}

// NewGetTradeMessagesUsecase は GetTradeMessagesUsecase を生成する。
func NewGetTradeMessagesUsecase(
	itemRepo *repository.ItemRepository,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeItemRepo *repository.TradeItemRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
	tradeMessageReadRepo *repository.TradeMessageReadRepository,
	userRepo *repository.UserRepository,
) *GetTradeMessagesUsecase {
	return &GetTradeMessagesUsecase{
		itemRepo:             itemRepo,
		messageConsentRepo:   messageConsentRepo,
		tradeRepo:            tradeRepo,
		tradeEventRepo:       tradeEventRepo,
		tradeItemRepo:        tradeItemRepo,
		tradeMessageRepo:     tradeMessageRepo,
		tradeMessageReadRepo: tradeMessageReadRepo,
		userRepo:             userRepo,
	}
}

// GetTradeMessagesInput は GetTradeMessagesUsecase.Execute の入力。
type GetTradeMessagesInput struct {
	// ViewerUserID はメッセージのページを開いたユーザー。
	ViewerUserID model.UserID
	TradeID      model.TradeID
}

// GetTradeMessagesOutput は GetTradeMessagesUsecase.Execute の結果。
type GetTradeMessagesOutput struct {
	Trade *model.Trade
	// Partner は交換の2人のうち、開いたユーザーの相手。退会していても返す。
	Partner *model.User
	// Items は交換の品のアイテムで、上の行の点数に使う。渡す人はアイテムの持ち主で決まる。
	Items []*model.Item
	// Messages は交換のメッセージで、送った順に並ぶ。取り消したものも含む。
	Messages []*model.TradeMessage
	// Events は交換の出来事で、起きた順に並ぶ。
	Events []*model.TradeEvent
	// MessageConsentValid は、開いたユーザーにメッセージの取り扱いへの有効な同意があるか。無ければ送る代わりに同意の案内を出す。
	MessageConsentValid bool
	// LastReadPosition は、開いたユーザーが前にこの交換のメッセージをどこまで読んだか。未読の境目に使う。読んだことが無ければnil。
	LastReadPosition *model.TradeMessageReadPosition
}

// Execute は交換 TradeID のメッセージを、開いたユーザーから見た形で返す。
//
// 交換が無いときと、開いたユーザーが交換の2人のどちらでもないときは、交換があることも知らせないため、
// どちらも AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetTradeMessagesUsecase) Execute(ctx context.Context, input GetTradeMessagesInput) (*GetTradeMessagesOutput, error) {
	trade, err := uc.tradeRepo.FindByID(ctx, input.TradeID)
	if err != nil {
		return nil, fmt.Errorf("交換の取得に失敗: %w", err)
	}
	if trade == nil || !policy.NewTradePolicy(input.ViewerUserID, trade).CanView() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"trade_id": input.TradeID.String(), "user_id": input.ViewerUserID.String()}}
	}

	// 交換の相手は、交換の記録を残すため外部キーで消せず、退会しても行が残るため、必ずある。
	partners, err := uc.userRepo.ListByIDs(ctx, []model.UserID{trade.PartnerUserID(input.ViewerUserID)})
	if err != nil {
		return nil, fmt.Errorf("交換の相手の取得に失敗: %w", err)
	}
	if len(partners) != 1 {
		return nil, fmt.Errorf("交換の相手の取得件数が不正: %d", len(partners))
	}

	itemsByTrade, err := findTradeItems(ctx, uc.tradeItemRepo, uc.itemRepo, []model.TradeID{trade.ID})
	if err != nil {
		return nil, err
	}

	messages, err := uc.tradeMessageRepo.ListByTradeID(ctx, trade.ID)
	if err != nil {
		return nil, fmt.Errorf("交換のメッセージの取得に失敗: %w", err)
	}
	events, err := uc.tradeEventRepo.ListByTradeID(ctx, trade.ID)
	if err != nil {
		return nil, fmt.Errorf("交換の出来事の取得に失敗: %w", err)
	}

	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.ViewerUserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}

	lastReadPosition, err := uc.tradeMessageReadRepo.FindLastReadPosition(ctx, trade.ID, input.ViewerUserID)
	if err != nil {
		return nil, fmt.Errorf("交換のメッセージをどこまで読んだかの取得に失敗: %w", err)
	}

	return &GetTradeMessagesOutput{
		Trade:               trade,
		Partner:             partners[0],
		Items:               itemsByTrade[trade.ID],
		Messages:            messages,
		Events:              events,
		MessageConsentValid: consent != nil && consent.IsValid(),
		LastReadPosition:    lastReadPosition,
	}, nil
}
