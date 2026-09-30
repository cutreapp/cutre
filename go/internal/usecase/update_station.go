package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdateStationUsecase は管理画面で駅の都道府県・名前・並び順を更新する。
type UpdateStationUsecase struct {
	validator   *validator.StationUpdateValidator
	stationRepo *repository.StationRepository
}

// NewUpdateStationUsecase は UpdateStationUsecase を生成する。
func NewUpdateStationUsecase(validator *validator.StationUpdateValidator, stationRepo *repository.StationRepository) *UpdateStationUsecase {
	return &UpdateStationUsecase{validator: validator, stationRepo: stationRepo}
}

// UpdateStationInput は UpdateStationUsecase.Execute の入力。
type UpdateStationInput struct {
	User      *model.User
	StationID model.StationID
	// LockVersion は編集のフォームを開いたときの駅の版。
	LockVersion    int32
	PrefectureCode string
	Name           string
	Position       string
}

// Execute は駅を更新し、更新したユーザーと駅をログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、駅が無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を、フォームの誤りは *model.ValidationError を返す。
// フォームを開いたあとにほかの操作で先に更新されていた (版が一致しない) ときは、上書きせずに
// AppErrCodeConflict の *model.AppError を返す。
func (uc *UpdateStationUsecase) Execute(ctx context.Context, input UpdateStationInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, err := findUndeletedStation(ctx, uc.stationRepo, input.StationID); err != nil {
		return err
	}

	attrs, err := uc.validator.Validate(ctx, validator.StationUpdateValidatorInput{
		PrefectureCode: input.PrefectureCode,
		Name:           input.Name,
		Position:       input.Position,
	})
	if err != nil {
		return err
	}

	updated, err := uc.stationRepo.Update(ctx, input.StationID, input.LockVersion, *attrs)
	if err != nil {
		return fmt.Errorf("駅の更新に失敗: %w", err)
	}
	if !updated {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"station_id": input.StationID.String()}}
	}
	slog.InfoContext(ctx, "駅を更新しました", "user_id", input.User.ID, "station_id", input.StationID)

	return nil
}
