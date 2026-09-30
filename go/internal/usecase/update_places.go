package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdatePlacesUsecase はユーザーの交換場所 (選んだ駅と「ほかに出られるところ」) を保存する。
type UpdatePlacesUsecase struct {
	db               *sql.DB
	validator        *validator.PlaceUpdateValidator
	stationValidator *validator.PlaceStationUpdateValidator
	stationRepo      *repository.StationRepository
	userStationRepo  *repository.UserStationRepository
	userRepo         *repository.UserRepository
}

// NewUpdatePlacesUsecase は UpdatePlacesUsecase を生成する。
func NewUpdatePlacesUsecase(
	db *sql.DB,
	validator *validator.PlaceUpdateValidator,
	stationValidator *validator.PlaceStationUpdateValidator,
	stationRepo *repository.StationRepository,
	userStationRepo *repository.UserStationRepository,
	userRepo *repository.UserRepository,
) *UpdatePlacesUsecase {
	return &UpdatePlacesUsecase{
		db:               db,
		validator:        validator,
		stationValidator: stationValidator,
		stationRepo:      stationRepo,
		userStationRepo:  userStationRepo,
		userRepo:         userRepo,
	}
}

// UpdatePlacesInput は UpdatePlacesUsecase.Execute の入力。駅と「ほかに出られるところ」はフォームの値をそのまま受け取る。
type UpdatePlacesInput struct {
	UserID      model.UserID
	LockVersion int32
	StationIDs  []string
	PlaceNote   string
}

// Execute は交換場所を、選んだ駅と「ほかに出られるところ」で置き換える。
//
// フォームの誤りと、選べない駅 (無い・削除した・選んでいないのにアーカイブした) を選んだときは *model.ValidationError を返す。
// 画面を開いたあとの変更と競合したときは AppErrCodeConflict を返す。
func (uc *UpdatePlacesUsecase) Execute(ctx context.Context, input UpdatePlacesInput) error {
	attrs, err := uc.validator.Validate(ctx, validator.PlaceUpdateValidatorInput{StationIDs: input.StationIDs, PlaceNote: input.PlaceNote})
	if err != nil {
		return err
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 同じユーザーの保存を直列化し、駅の集合を置き換える前にフォームの版を確かめる。
	// 退会と重なった場合も、ロック取得後の状態を基準にする。
	userRepo := uc.userRepo.WithTx(tx)
	user, err := userRepo.LockByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("ユーザーのロックに失敗: %w", err)
	}
	if user == nil {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}
	if user.PlaceLockVersion != input.LockVersion {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"user_id": input.UserID.String()}}
	}

	// 駅の削除と直列化するため、選んだ駅の行をロックしてから状態を確かめる。
	// 削除が先に終われば選べない駅として拒否し、あとなら削除の側が参照を見つけてアーカイブを案内する。
	if err := uc.stationRepo.WithTx(tx).LockByIDs(ctx, attrs.StationIDs); err != nil {
		return fmt.Errorf("駅のロックに失敗: %w", err)
	}
	if err := uc.stationValidator.WithTx(tx).Validate(ctx, input.UserID, attrs.StationIDs); err != nil {
		return err
	}

	if err := uc.userStationRepo.WithTx(tx).Replace(ctx, input.UserID, attrs.StationIDs); err != nil {
		return fmt.Errorf("交換場所の駅の保存に失敗: %w", err)
	}
	updated, err := userRepo.UpdatePlaces(ctx, input.UserID, attrs.PlaceNote, input.LockVersion)
	if err != nil {
		return fmt.Errorf("ほかに出られるところの保存に失敗: %w", err)
	}
	if !updated {
		return &model.AppError{Code: model.AppErrCodeConflict, Metadata: map[string]string{"user_id": input.UserID.String()}}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
