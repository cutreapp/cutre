package validator

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// StationDeleteValidator は管理画面で駅を削除できるかを検証する。
type StationDeleteValidator struct {
	userStationRepo *repository.UserStationRepository
}

// NewStationDeleteValidator は StationDeleteValidator を生成する。
func NewStationDeleteValidator(userStationRepo *repository.UserStationRepository) *StationDeleteValidator {
	return &StationDeleteValidator{userStationRepo: userStationRepo}
}

// WithTx はtx内で交換場所の参照を確認する新しいValidatorを返す。
func (v *StationDeleteValidator) WithTx(tx *sql.Tx) *StationDeleteValidator {
	return &StationDeleteValidator{userStationRepo: v.userStationRepo.WithTx(tx)}
}

// Validate は、駅を交換場所に選んでいるユーザーがいないかを確かめる。
// 参照があるときは削除させず、代わりにアーカイブを案内する *model.ValidationError を返す。
func (v *StationDeleteValidator) Validate(ctx context.Context, id model.StationID) error {
	referenced, err := v.userStationRepo.ExistsByStationID(ctx, id)
	if err != nil {
		return fmt.Errorf("駅を選んでいる交換場所の確認に失敗: %w", err)
	}
	if referenced {
		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "admin_station_delete_referenced"))
		return ve
	}

	return nil
}
