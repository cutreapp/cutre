package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetPlacesUsecase はユーザーが交換場所に選んだ駅を引く。
type GetPlacesUsecase struct {
	stationRepo *repository.StationRepository
	userRepo    *repository.UserRepository
}

// NewGetPlacesUsecase は GetPlacesUsecase を生成する。
func NewGetPlacesUsecase(stationRepo *repository.StationRepository, userRepo *repository.UserRepository) *GetPlacesUsecase {
	return &GetPlacesUsecase{stationRepo: stationRepo, userRepo: userRepo}
}

// GetPlacesInput は GetPlacesUsecase.Execute の入力。
type GetPlacesInput struct {
	UserID model.UserID
}

// GetPlacesOutput は GetPlacesUsecase.Execute の結果。
type GetPlacesOutput struct {
	// User は「ほかに出られるところ」と交換場所の版を含む現在のユーザー。
	User *model.User
	// Stations は交換場所に選んだ駅。都道府県コードの順、都道府県の中では並び順に並ぶ。
	// 選んだあとにアーカイブした駅も含む。
	Stations []*model.Station
}

// Execute はユーザーが交換場所に選んだ駅を返す。
func (uc *GetPlacesUsecase) Execute(ctx context.Context, input GetPlacesInput) (*GetPlacesOutput, error) {
	user, err := uc.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	stations, err := uc.stationRepo.ListByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("交換場所の駅の取得に失敗: %w", err)
	}

	return &GetPlacesOutput{User: user, Stations: stations}, nil
}
