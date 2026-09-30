package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetMessagesUsecase は、メッセージの一覧に出す、ユーザーの交換を、相手・交換の品・最新のメッセージ・未読の数と一緒に引く。
type GetMessagesUsecase struct {
	itemRepo         *repository.ItemRepository
	tradeRepo        *repository.TradeRepository
	tradeItemRepo    *repository.TradeItemRepository
	tradeMessageRepo *repository.TradeMessageRepository
	userRepo         *repository.UserRepository
}

// NewGetMessagesUsecase は GetMessagesUsecase を生成する。
func NewGetMessagesUsecase(
	itemRepo *repository.ItemRepository,
	tradeRepo *repository.TradeRepository,
	tradeItemRepo *repository.TradeItemRepository,
	tradeMessageRepo *repository.TradeMessageRepository,
	userRepo *repository.UserRepository,
) *GetMessagesUsecase {
	return &GetMessagesUsecase{
		itemRepo:         itemRepo,
		tradeRepo:        tradeRepo,
		tradeItemRepo:    tradeItemRepo,
		tradeMessageRepo: tradeMessageRepo,
		userRepo:         userRepo,
	}
}

// GetMessagesInput は GetMessagesUsecase.Execute の入力。
type GetMessagesInput struct {
	UserID model.UserID
}

// GetMessagesOutput は GetMessagesUsecase.Execute の結果。
type GetMessagesOutput struct {
	// Trades はユーザーの交換で、終わったものも含む。最新のメッセージが新しいものから並ぶ。
	Trades []*model.Trade
	// Partners は交換の相手。退会した相手も入る。
	Partners map[model.UserID]*model.User
	// Items は交換ごとの品のアイテムで、入れた順に1点ずつ並ぶ。渡す人はアイテムの持ち主で決まる。
	Items map[model.TradeID][]*model.Item
	// LatestMessages は交換ごとの最新のメッセージ。取り消したものも入り、メッセージの無い交換は入らない。
	LatestMessages map[model.TradeID]*model.TradeMessage
	// UnreadCounts は交換ごとの未読のメッセージの数。未読の無い交換は入らない。
	UnreadCounts map[model.TradeID]int64
}

// Execute はユーザーの交換を、メッセージの一覧に出す形で返す。
//
// 相手・交換の品・最新のメッセージ・未読の数は、交換の数によらずクエリの回数を一定にしてまとめて引く。
// 招待制のあいだは件数が多くならないため、ページに分けない。
func (uc *GetMessagesUsecase) Execute(ctx context.Context, input GetMessagesInput) (*GetMessagesOutput, error) {
	trades, err := uc.tradeRepo.ListByUserIDOrderByLatestMessage(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("交換の取得に失敗: %w", err)
	}

	tradeIDs := make([]model.TradeID, len(trades))
	partnerIDs := make([]model.UserID, len(trades))
	for i, trade := range trades {
		tradeIDs[i] = trade.ID
		partnerIDs[i] = trade.PartnerUserID(input.UserID)
	}

	partnerList, err := uc.userRepo.ListByIDs(ctx, partnerIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の相手の取得に失敗: %w", err)
	}
	partners := make(map[model.UserID]*model.User, len(partnerList))
	for _, partner := range partnerList {
		partners[partner.ID] = partner
	}

	items, err := findTradeItems(ctx, uc.tradeItemRepo, uc.itemRepo, tradeIDs)
	if err != nil {
		return nil, err
	}

	latestMessages, err := uc.tradeMessageRepo.ListLatestByTradeIDs(ctx, tradeIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の最新のメッセージの取得に失敗: %w", err)
	}
	unreadCounts, err := uc.tradeMessageRepo.CountUnreadByTradeIDs(ctx, input.UserID, tradeIDs)
	if err != nil {
		return nil, fmt.Errorf("交換の未読のメッセージの数の取得に失敗: %w", err)
	}

	return &GetMessagesOutput{
		Trades:         trades,
		Partners:       partners,
		Items:          items,
		LatestMessages: latestMessages,
		UnreadCounts:   unreadCounts,
	}, nil
}
