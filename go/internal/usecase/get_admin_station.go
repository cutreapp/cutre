package usecase

import (
	"context"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/policy"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetAdminStationUsecase は管理画面で編集・アーカイブする駅を引く。
type GetAdminStationUsecase struct {
	stationRepo *repository.StationRepository
}

// NewGetAdminStationUsecase は GetAdminStationUsecase を生成する。
func NewGetAdminStationUsecase(stationRepo *repository.StationRepository) *GetAdminStationUsecase {
	return &GetAdminStationUsecase{stationRepo: stationRepo}
}

// GetAdminStationInput は GetAdminStationUsecase.Execute の入力。
type GetAdminStationInput struct {
	User      *model.User
	StationID model.StationID
}

// GetAdminStationOutput は GetAdminStationUsecase.Execute の結果。
type GetAdminStationOutput struct {
	Station *model.Station
	// CanDelete はユーザーがこの駅を削除できるか。削除の欄を出すかを決める。
	CanDelete bool
}

// Execute は駅を返す。
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、
// 駅が無いか削除したときは AppErrCodeResourceNotFound の *model.AppError を返す。
func (uc *GetAdminStationUsecase) Execute(ctx context.Context, input GetAdminStationInput) (*GetAdminStationOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	station, err := findUndeletedStation(ctx, uc.stationRepo, input.StationID)
	if err != nil {
		return nil, err
	}

	return &GetAdminStationOutput{
		Station:   station,
		CanDelete: policy.NewAdminPolicy(input.User).CanDeleteMaster(),
	}, nil
}
