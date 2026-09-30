package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetPublishedStationsUsecase は交換場所の選択肢にする、公開中の駅を引く。
type GetPublishedStationsUsecase struct {
	stationRepo *repository.StationRepository
}

// NewGetPublishedStationsUsecase は GetPublishedStationsUsecase を生成する。
func NewGetPublishedStationsUsecase(stationRepo *repository.StationRepository) *GetPublishedStationsUsecase {
	return &GetPublishedStationsUsecase{stationRepo: stationRepo}
}

// GetPublishedStationsOutput は GetPublishedStationsUsecase.Execute の結果。
type GetPublishedStationsOutput struct {
	// Stations は公開中の駅。都道府県コードの順、都道府県の中では並び順に並ぶ。
	Stations []*model.Station
}

// Execute は公開中の駅を返す。
func (uc *GetPublishedStationsUsecase) Execute(ctx context.Context) (*GetPublishedStationsOutput, error) {
	stations, err := uc.stationRepo.ListPublished(ctx)
	if err != nil {
		return nil, fmt.Errorf("公開中の駅の取得に失敗: %w", err)
	}

	return &GetPublishedStationsOutput{Stations: stations}, nil
}
