package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminStationsUsecase は管理画面の駅の一覧を引く。
type GetAdminStationsUsecase struct {
	stationRepo *repository.StationRepository
}

// NewGetAdminStationsUsecase は GetAdminStationsUsecase を生成する。
func NewGetAdminStationsUsecase(stationRepo *repository.StationRepository) *GetAdminStationsUsecase {
	return &GetAdminStationsUsecase{stationRepo: stationRepo}
}

// GetAdminStationsInput は GetAdminStationsUsecase.Execute の入力。
type GetAdminStationsInput struct {
	User *model.User
}

// GetAdminStationsOutput は GetAdminStationsUsecase.Execute の結果。
type GetAdminStationsOutput struct {
	// Stations は削除していない駅を、都道府県コードの順、都道府県の中では並び順に並べたもの。
	Stations []*model.Station
}

// Execute は削除していない駅を返す。
// 駅は都道府県ごとの主要駅に限り多くならないため、ページに分けずにすべて返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の *model.AppError を返す。
func (uc *GetAdminStationsUsecase) Execute(ctx context.Context, input GetAdminStationsInput) (*GetAdminStationsOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	stations, err := uc.stationRepo.ListUndeleted(ctx)
	if err != nil {
		return nil, fmt.Errorf("駅の一覧の取得に失敗: %w", err)
	}

	return &GetAdminStationsOutput{Stations: stations}, nil
}
