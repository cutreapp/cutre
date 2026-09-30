package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// findUndeletedStation は管理画面で扱う駅 (公開中かアーカイブしたもの) を返す。
// 無いときと削除したときは、管理画面にも出さないため AppErrCodeResourceNotFound の *model.AppError を返す。
func findUndeletedStation(ctx context.Context, stationRepo *repository.StationRepository, id model.StationID) (*model.Station, error) {
	station, err := stationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("駅の取得に失敗: %w", err)
	}
	if station == nil || station.IsDeleted() {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound, Metadata: map[string]string{"station_id": id.String()}}
	}

	return station, nil
}
