package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// ArchiveStationUsecase は管理画面で公開中の駅を、理由を残してアーカイブする。
type ArchiveStationUsecase struct {
	validator   *validator.StationArchiveCreateValidator
	stationRepo *repository.StationRepository
}

// NewArchiveStationUsecase は ArchiveStationUsecase を生成する。
func NewArchiveStationUsecase(validator *validator.StationArchiveCreateValidator, stationRepo *repository.StationRepository) *ArchiveStationUsecase {
	return &ArchiveStationUsecase{validator: validator, stationRepo: stationRepo}
}

// ArchiveStationInput は ArchiveStationUsecase.Execute の入力。
type ArchiveStationInput struct {
	User           *model.User
	StationID      model.StationID
	LockVersion    int32
	ArchiveMessage string
}

// Execute は駅をアーカイブし、アーカイブしたユーザーと駅をログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、駅が無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、理由の誤りは *model.ValidationError を返す。
// 既にアーカイブしていたときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *ArchiveStationUsecase) Execute(ctx context.Context, input ArchiveStationInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	station, err := findUndeletedStation(ctx, uc.stationRepo, input.StationID)
	if err != nil {
		return err
	}
	if station.IsArchived() {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"station_id": input.StationID.String()}}
	}

	message, err := uc.validator.Validate(ctx, validator.StationArchiveCreateValidatorInput{ArchiveMessage: input.ArchiveMessage})
	if err != nil {
		return err
	}

	archived, err := uc.stationRepo.Archive(ctx, input.StationID, input.LockVersion, message)
	if err != nil {
		return fmt.Errorf("駅のアーカイブに失敗: %w", err)
	}
	if !archived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"station_id": input.StationID.String()}}
	}
	slog.InfoContext(ctx, "駅をアーカイブしました", "user_id", input.User.ID, "station_id", input.StationID)

	return nil
}
