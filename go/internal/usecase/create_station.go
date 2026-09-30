package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateStationUsecase は管理画面で駅を作成する。
type CreateStationUsecase struct {
	validator   *validator.StationCreateValidator
	stationRepo *repository.StationRepository
}

// NewCreateStationUsecase は CreateStationUsecase を生成する。
func NewCreateStationUsecase(validator *validator.StationCreateValidator, stationRepo *repository.StationRepository) *CreateStationUsecase {
	return &CreateStationUsecase{validator: validator, stationRepo: stationRepo}
}

// CreateStationInput は CreateStationUsecase.Execute の入力。都道府県と並び順はフォームの値をそのまま受け取る。
type CreateStationInput struct {
	User           *model.User
	PrefectureCode string
	Name           string
	Position       string
}

// CreateStationOutput は CreateStationUsecase.Execute の結果。
type CreateStationOutput struct {
	Station *model.Station
}

// Execute は公開中の駅を作成し、作成したユーザーと駅をログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の *model.AppError を、
// フォームの誤りは *model.ValidationError を返す。
func (uc *CreateStationUsecase) Execute(ctx context.Context, input CreateStationInput) (*CreateStationOutput, error) {
	if err := authorizeAdmin(input.User); err != nil {
		return nil, err
	}

	attrs, err := uc.validator.Validate(ctx, validator.StationCreateValidatorInput{
		PrefectureCode: input.PrefectureCode,
		Name:           input.Name,
		Position:       input.Position,
	})
	if err != nil {
		return nil, err
	}

	station, err := uc.stationRepo.Create(ctx, *attrs)
	if err != nil {
		return nil, fmt.Errorf("駅の作成に失敗: %w", err)
	}
	slog.InfoContext(ctx, "駅を作成しました", "user_id", input.User.ID, "station_id", station.ID)

	return &CreateStationOutput{Station: station}, nil
}
