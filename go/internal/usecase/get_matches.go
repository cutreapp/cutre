package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetMatchesUsecase はユーザーのマッチ候補を、交換できるアイテムと交換場所と一緒に引く。
type GetMatchesUsecase struct {
	eventCategoryRepo *repository.EventCategoryRepository
	goodsRepo         *repository.GoodsRepository
	itemRepo          *repository.ItemRepository
	stationRepo       *repository.StationRepository
	userRepo          *repository.UserRepository
	userStationRepo   *repository.UserStationRepository
}

// NewGetMatchesUsecase は GetMatchesUsecase を生成する。
func NewGetMatchesUsecase(
	eventCategoryRepo *repository.EventCategoryRepository,
	goodsRepo *repository.GoodsRepository,
	itemRepo *repository.ItemRepository,
	stationRepo *repository.StationRepository,
	userRepo *repository.UserRepository,
	userStationRepo *repository.UserStationRepository,
) *GetMatchesUsecase {
	return &GetMatchesUsecase{
		eventCategoryRepo: eventCategoryRepo,
		goodsRepo:         goodsRepo,
		itemRepo:          itemRepo,
		stationRepo:       stationRepo,
		userRepo:          userRepo,
		userStationRepo:   userStationRepo,
	}
}

// GetMatchesInput は GetMatchesUsecase.Execute の入力。
type GetMatchesInput struct {
	UserID model.UserID
}

// GetMatchesOutput は GetMatchesUsecase.Execute の結果。
type GetMatchesOutput struct {
	// Candidates はマッチ候補のユーザー。アットネームの順に並ぶ。
	Candidates []*model.User
	// Matches は候補ごとの、ユーザーとの間で交換できるアイテム。
	Matches map[model.UserID]model.Match
	// Stations は候補ごとの交換場所の駅。都道府県コードの順、都道府県の中では並び順に並ぶ。
	Stations map[model.UserID][]*model.Station
	// Goods・EventCategories は、交換できるアイテムのグッズと、そのカテゴリー。マスタの状態は問わない。
	Goods           map[model.GoodsID]*model.Goods
	EventCategories map[model.EventCategoryID]*model.EventCategory
	// HasPlaces はユーザーが交換場所の駅を1つ以上選んでいるか。候補がいないときの案内を選ぶのに使う。
	HasPlaces bool
}

// Execute はユーザーのマッチ候補と、候補ごとの交換できるアイテム・交換場所を返す。
//
// 招待制のあいだは人数が少ないため、事前に計算しておかず、画面を開くたびに求める。ページにも分けない。
func (uc *GetMatchesUsecase) Execute(ctx context.Context, input GetMatchesInput) (*GetMatchesOutput, error) {
	candidates, err := uc.userRepo.ListMatchCandidates(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("マッチ候補の取得に失敗: %w", err)
	}

	candidateIDs := make([]model.UserID, len(candidates))
	for i, candidate := range candidates {
		candidateIDs[i] = candidate.ID
	}
	matches, err := findMatches(ctx, uc.itemRepo, uc.goodsRepo, uc.eventCategoryRepo, input.UserID, candidateIDs)
	if err != nil {
		return nil, err
	}

	stations, err := uc.stationRepo.ListByUserIDs(ctx, candidateIDs)
	if err != nil {
		return nil, fmt.Errorf("マッチ候補の交換場所の取得に失敗: %w", err)
	}

	hasPlaces, err := uc.userStationRepo.ExistsByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("交換場所の有無の確認に失敗: %w", err)
	}

	return &GetMatchesOutput{
		Candidates:      candidates,
		Matches:         matches.Matches,
		Stations:        stations,
		Goods:           matches.Goods,
		EventCategories: matches.EventCategories,
		HasPlaces:       hasPlaces,
	}, nil
}
