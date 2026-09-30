package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// DeleteStationUsecase は管理画面で駅を削除する。
// 行は消さずに削除した状態にし、管理画面にも出さなくする。物理削除はあとからまとめて行う。
type DeleteStationUsecase struct {
	db          *sql.DB
	validator   *validator.StationDeleteValidator
	stationRepo *repository.StationRepository
}

// NewDeleteStationUsecase は DeleteStationUsecase を生成する。
func NewDeleteStationUsecase(db *sql.DB, validator *validator.StationDeleteValidator, stationRepo *repository.StationRepository) *DeleteStationUsecase {
	return &DeleteStationUsecase{db: db, validator: validator, stationRepo: stationRepo}
}

// DeleteStationInput は DeleteStationUsecase.Execute の入力。
type DeleteStationInput struct {
	User        *model.User
	StationID   model.StationID
	LockVersion int32
}

// Execute は駅を削除し、削除したユーザーと駅をログに残す。
//
// ユーザーが管理者でないときは AppErrCodeForbidden の、駅が無いか削除したときは
// AppErrCodeResourceNotFound の *model.AppError を返す。
// 駅を交換場所に選んでいるユーザーがいるときは、アーカイブを案内する *model.ValidationError を返す。
// 画面を開いたあとに版が変わったときは AppErrCodeConflict を返す。
func (uc *DeleteStationUsecase) Execute(ctx context.Context, input DeleteStationInput) error {
	if err := authorizeMasterDeletion(input.User); err != nil {
		return err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stationRepo := uc.stationRepo.WithTx(tx)

	// 交換場所の保存と直列化するため、駅の行をロックしてから参照を確かめる。
	if err := stationRepo.LockByID(ctx, input.StationID); err != nil {
		return fmt.Errorf("駅のロックに失敗: %w", err)
	}
	if _, err := findUndeletedStation(ctx, stationRepo, input.StationID); err != nil {
		return err
	}
	if err := uc.validator.WithTx(tx).Validate(ctx, input.StationID); err != nil {
		return err
	}

	deleted, err := stationRepo.Delete(ctx, input.StationID, input.LockVersion)
	if err != nil {
		return fmt.Errorf("駅の削除に失敗: %w", err)
	}
	if !deleted {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"station_id": input.StationID.String()}}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	slog.InfoContext(ctx, "駅を削除しました", "user_id", input.User.ID, "station_id", input.StationID)

	return nil
}
