package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetProfileUsecase は、ほかのユーザーのプロフィールに出す交換場所・「交換できた」の件数と、閲覧しているユーザーとの間で交換できるアイテムを引く。
// 自分のプロフィール (マイページ) には使わない。
type GetProfileUsecase struct {
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
	stationRepo       *repository.StationRepository
	tradeRepo         *repository.TradeRepository
	userRepo          *repository.UserRepository
}

// NewGetProfileUsecase は GetProfileUsecase を生成する。
func NewGetProfileUsecase(
	eventCategoryRepo *repository.EventCategoryRepository,
	goodsRepo *repository.GoodsRepository,
	itemRepo *repository.ItemRepository,
	stationRepo *repository.StationRepository,
	tradeRepo *repository.TradeRepository,
	userRepo *repository.UserRepository,
) *GetProfileUsecase {
	return &GetProfileUsecase{
		eventCategoryRepo: eventCategoryRepo,
		goodsRepo:         goodsRepo,
		itemRepo:          itemRepo,
		stationRepo:       stationRepo,
		tradeRepo:         tradeRepo,
		userRepo:          userRepo,
	}
}

// GetProfileInput は GetProfileUsecase.Execute の入力。
type GetProfileInput struct {
	// ViewerUserID はプロフィールを見ているユーザー。
	ViewerUserID model.UserID
	// Atname はプロフィールのユーザーのアットネーム。大文字小文字を区別しない。
	Atname string
}

// GetProfileOutput は GetProfileUsecase.Execute の結果。
type GetProfileOutput struct {
	// User はプロフィールのユーザー。「ほかに出られるところ」を含む。
	User *model.User
	// Stations はプロフィールのユーザーの交換場所の駅。都道府県コードの順、都道府県の中では並び順に並ぶ。
	Stations []*model.Station
	// CompletedTradeCount は、プロフィールのユーザーが「交換できた」で終えた交換の数。相手は問わない。
	CompletedTradeCount int64
	// Match は見ているユーザーとの間で交換できるアイテム。交換場所の都道府県が同じかは問わない。
	Match model.Match
	// Goods・EventCategories は、交換できるアイテムのグッズと、そのカテゴリー。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
}

// Execute はアットネーム Atname のユーザーのプロフィールを返す。
//
// ユーザーがいないか退会したときと、見ているユーザー自身のときは AppErrCodeResourceNotFound の *model.AppError を返す。
// 自分のプロフィールはマイページとして別に描くため。
func (uc *GetProfileUsecase) Execute(ctx context.Context, input GetProfileInput) (*GetProfileOutput, error) {
	user, err := uc.userRepo.FindByAtname(ctx, input.Atname)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil || user.ID == input.ViewerUserID {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"atname": input.Atname}}
	}

	stations, err := uc.stationRepo.ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("交換場所の駅の取得に失敗: %w", err)
	}

	endedCounts, err := uc.tradeRepo.CountEndedByUserIDGroupByStatus(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("終わった交換の数の取得に失敗: %w", err)
	}

	matches, err := findMatches(ctx, uc.itemRepo, uc.goodsRepo, uc.eventCategoryRepo, input.ViewerUserID, []model.UserID{user.ID})
	if err != nil {
		return nil, err
	}

	return &GetProfileOutput{
		User:                user,
		Stations:            stations,
		CompletedTradeCount: endedCounts[model.TradeStatusCompleted],
		Match:               matches.Matches[user.ID],
		Goods:               matches.Goods,
		EventCategories:     matches.EventCategories,
	}, nil
}
