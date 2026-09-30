package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetTradeUsecase は交換のページに出す、交換とその相手・交換の品・これまでの流れを引く。
type GetTradeUsecase struct {
	eventCategoryRepo  *repository.EventCategoryRepository
	goodsRepo          *repository.GoodsRepository
	itemRepo           *repository.ItemRepository
	messageConsentRepo *repository.MessageConsentRepository
	tradeRepo          *repository.TradeRepository
	tradeEventRepo     *repository.TradeEventRepository
	tradeItemRepo      *repository.TradeItemRepository
	tradeMessageRepo   *repository.TradeMessageRepository
	userRepo           *repository.UserRepository
}

// NewGetTradeUsecase は GetTradeUsecase を生成する。
func NewGetTradeUsecase(
	eventCategoryRepo *repository.EventCategoryRepository,
	goodsRepo *repository.GoodsRepository,
	itemRepo *repository.ItemRepository,
	messageConsentRepo *repository.MessageConsentRepository,
	tradeRepo *repository.TradeRepository,
	tradeEventRepo *repository.TradeEventRepository,
	tradeItemRepo *repository.TradeItemRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
	userRepo *repository.UserRepository,
) *GetTradeUsecase {
	return &GetTradeUsecase{
		eventCategoryRepo:  eventCategoryRepo,
		goodsRepo:          goodsRepo,
		itemRepo:           itemRepo,
		messageConsentRepo: messageConsentRepo,
		tradeRepo:          tradeRepo,
		tradeEventRepo:     tradeEventRepo,
		tradeItemRepo:      tradeItemRepo,
		tradeMessageRepo:   tradeMessageRepo,
		userRepo:           userRepo,
	}
}

// GetTradeInput は GetTradeUsecase.Execute の入力。
type GetTradeInput struct {
	// ViewerUserID は交換のページを開いたユーザー。
	ViewerUserID model.UserID
	TradeID      model.TradeID
}

// GetTradeOutput は GetTradeUsecase.Execute の結果。
type GetTradeOutput struct {
	Trade *model.Trade
	// Partner は交換の2人のうち、開いたユーザーの相手。退会していても返す。
	Partner *model.User
	// Items は交換の品のアイテムで、入れた順に1点ずつ並ぶ。渡す人はアイテムの持ち主で決まる。
	// リストから外したアイテムも、交換の記録として返す。
	Items []*model.Item
	// Goods・EventCategories は、交換の品のグッズと、そのカテゴリー。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
	// Events は交換の出来事で、起きた順に並ぶ。
	Events []*model.TradeEvent
	// LatestMessage は交換の最新のメッセージで、メッセージの行に出す。取り消したものも返し、メッセージが無ければnil。
	LatestMessage *model.TradeMessage
	// UnreadMessageCount は、開いたユーザーの未読のメッセージの数。
	UnreadMessageCount int64
	// MessageConsentValid は、開いたユーザーにメッセージの取り扱いへの有効な同意があるか。
	// 無ければ承認の代わりに同意の案内を出し、お断りではひとことの欄を出さない。
	MessageConsentValid bool
}

// Execute は交換 TradeID を、開いたユーザーから見た形で返す。
//
// 交換が無いときと、開いたユーザーが交換の2人のどちらでもないときは、交換があることも知らせないため、
// どちらも AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetTradeUsecase) Execute(ctx context.Context, input GetTradeInput) (*GetTradeOutput, error) {
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
	items := itemsByTrade[trade.ID]
	goodsIDs := make([]model.GoodsID, len(items))
	for i, item := range items {
		goodsIDs[i] = item.GoodsID
	}
	goods, categories, err := findGoodsWithCategories(ctx, uc.goodsRepo, uc.eventCategoryRepo, goodsIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の品の名前の取得に失敗: %w", err)
	}

	events, err := uc.tradeEventRepo.ListByTradeID(ctx, trade.ID)
	if err != nil {
		return nil, fmt.Errorf("交換の出来事の取得に失敗: %w", err)
	}

	latestMessages, err := uc.tradeMessageRepo.ListLatestByTradeIDs(ctx, []model.TradeID{trade.ID})
	if err != nil {
		return nil, fmt.Errorf("交換の最新のメッセージの取得に失敗: %w", err)
	}
	unreadCounts, err := uc.tradeMessageRepo.CountUnreadByTradeIDs(ctx, input.ViewerUserID, []model.TradeID{trade.ID})
	if err != nil {
		return nil, fmt.Errorf("交換の未読のメッセージの数の取得に失敗: %w", err)
	}

	consent, err := uc.messageConsentRepo.FindLatestByUserID(ctx, input.ViewerUserID)
	if err != nil {
		return nil, fmt.Errorf("メッセージの取り扱いへの同意の取得に失敗: %w", err)
	}

	return &GetTradeOutput{
		Trade:               trade,
		Partner:             partners[0],
		Items:               items,
		Goods:               goods,
		EventCategories:     categories,
		Events:              events,
		LatestMessage:       latestMessages[trade.ID],
		UnreadMessageCount:  unreadCounts[trade.ID],
		MessageConsentValid: consent != nil && consent.IsValid(),
	}, nil
}

// findTradeItems は、交換 tradeIDs ごとの品のアイテムを、入れた順に1点ずつ返す。
// 交換の品と、それが指すアイテムを、交換の数によらずそれぞれ1回のクエリでまとめて引く。
// リストから外したアイテムも、交換の記録として返す。
func findTradeItems(
	ctx context.Context,
	tradeItemRepo *repository.TradeItemRepository,
	itemRepo *repository.ItemRepository,
	tradeIDs []model.TradeID,
) (map[model.TradeID][]*model.Item, error) {
	itemIDs, err := tradeItemRepo.ListItemIDsByTradeIDs(ctx, tradeIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の品の取得に失敗: %w", err)
	}
	var allItemIDs []model.ItemID
	for _, ids := range itemIDs {
		allItemIDs = append(allItemIDs, ids...)
	}
	found, err := itemRepo.ListByIDs(ctx, allItemIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の品のアイテムの取得に失敗: %w", err)
	}
	byID := make(map[model.ItemID]*model.Item, len(found))
	for _, item := range found {
		byID[item.ID] = item
	}

	// 交換の品が指すアイテムは外部キーで消せないため、必ずある。
	items := make(map[model.TradeID][]*model.Item, len(itemIDs))
	for tradeID, ids := range itemIDs {
		for _, id := range ids {
			if item := byID[id]; item != nil {
				items[tradeID] = append(items[tradeID], item)
			}
		}
	}

	return items, nil
}
