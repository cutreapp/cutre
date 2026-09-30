package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// UnarchiveStationUsecase は管理画面でアーカイブした駅を公開に戻す。
type UnarchiveStationUsecase struct {
	stationRepo *repository.StationRepository
}

// NewUnarchiveStationUsecase は UnarchiveStationUsecase を生成する。
func NewUnarchiveStationUsecase(stationRepo *repository.StationRepository) *UnarchiveStationUsecase {
	return &UnarchiveStationUsecase{stationRepo: stationRepo}
}

// UnarchiveStationInput は UnarchiveStationUsecase.Execute の入力。
type UnarchiveStationInput struct {
	User        *model.User
	StationID   model.StationID
	LockVersion int32
}

// Execute は駅を公開に戻して理由を空にし、戻したユーザーと駅をログに残す。
//
// ユーザーが管理画面を使えないときは AppErrCodeForbidden の、駅が無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// アーカイブしていなかったときと、画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *UnarchiveStationUsecase) Execute(ctx context.Context, input UnarchiveStationInput) error {
	if err := authorizeAdmin(input.User); err != nil {
		return err
	}

	if _, err := findUndeletedStation(ctx, uc.stationRepo, input.StationID); err != nil {
		return err
	}

	unarchived, err := uc.stationRepo.Unarchive(ctx, input.StationID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("駅を元に戻すのに失敗: %w", err)
	}
	if !unarchived {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"station_id": input.StationID.String()}}
	}
	slog.InfoContext(ctx, "駅を元に戻しました", "user_id", input.User.ID, "station_id", input.StationID)

	return nil
}
