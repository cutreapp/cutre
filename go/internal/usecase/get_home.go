package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetHomeUsecase はログイン後のホームに出す、ユーザーの状況を引く。
type GetHomeUsecase struct {
	itemRepo        *repository.ItemRepository
	userRepo        *repository.UserRepository
	userStationRepo *repository.UserStationRepository
}

// NewGetHomeUsecase は GetHomeUsecase を生成する。
func NewGetHomeUsecase(itemRepo *repository.ItemRepository, userRepo *repository.UserRepository, userStationRepo *repository.UserStationRepository) *GetHomeUsecase {
	return &GetHomeUsecase{itemRepo: itemRepo, userRepo: userRepo, userStationRepo: userStationRepo}
}

// GetHomeInput は GetHomeUsecase.Execute の入力。
type GetHomeInput struct {
	UserID model.UserID
}

// GetHomeOutput は GetHomeUsecase.Execute の結果。
type GetHomeOutput struct {
	// Quantities は、譲れる・ほしいのリストそれぞれのアイテムの数量の合計。
	Quantities model.ItemQuantities
	// HasPlaces は交換場所の駅を1つ以上選んでいるか。
	HasPlaces bool
	// MatchCount はマッチ候補の人数。
	MatchCount int
}

// Execute はユーザーのリストごとのアイテムの数量の合計と、交換場所を選んでいるかと、マッチ候補の人数を返す。
//
// 人数はマッチ候補の画面と同じ条件で数えるため、候補の一覧を引いて数える。招待制のあいだは候補が多くならない。
func (uc *GetHomeUsecase) Execute(ctx context.Context, input GetHomeInput) (*GetHomeOutput, error) {
	quantities, err := uc.itemRepo.SumListedQuantitiesByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("リストごとのアイテムの数量の取得に失敗: %w", err)
	}

	hasPlaces, err := uc.userStationRepo.ExistsByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("交換場所の有無の確認に失敗: %w", err)
	}

	candidates, err := uc.userRepo.ListMatchCandidates(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("マッチ候補の取得に失敗: %w", err)
	}

	return &GetHomeOutput{Quantities: quantities, HasPlaces: hasPlaces, MatchCount: len(candidates)}, nil
}
