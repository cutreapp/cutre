package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// UserStationRepository はuser_stations (ユーザーが交換場所に選んだ駅) を読み書きする。
// 選んだ駅そのものは StationRepository.ListByUserID で引く。
type UserStationRepository struct {
	q *query.Queries
}

// NewUserStationRepository は UserStationRepository を生成する。
func NewUserStationRepository(db *sql.DB) *UserStationRepository {
	return &UserStationRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserStationRepository を返す。
func (r *UserStationRepository) WithTx(tx *sql.Tx) *UserStationRepository {
	return &UserStationRepository{q: r.q.WithTx(tx)}
}

// ExistsByUserID は、ユーザーが交換場所を1つ以上選んでいるかを返す。
func (r *UserStationRepository) ExistsByUserID(ctx context.Context, userID model.UserID) (bool, error) {
	return r.q.ExistsUserStationByUserID(ctx, uuid.UUID(userID))
}

// ExistsByStationID は、駅 stationID を交換場所に選んでいるユーザーがいるかを返す。
func (r *UserStationRepository) ExistsByStationID(ctx context.Context, stationID model.StationID) (bool, error) {
	return r.q.ExistsUserStationByStationID(ctx, uuid.UUID(stationID))
}

// DeleteByUserID は、ユーザー userID の交換場所をすべて消す。退会で使う。
func (r *UserStationRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeleteUserStationsByUserID(ctx, uuid.UUID(userID))
}

// Replace はユーザーの交換場所を stationIDs の駅だけにする。
// stationIDs に無い駅を外し、まだ選んでいない駅を足す。選んだままの駅の行は作り直さない。
// 駅が選べるものかは呼び出し側が確かめる。
func (r *UserStationRepository) Replace(ctx context.Context, userID model.UserID, stationIDs []model.StationID) error {
	uuids := stationUUIDs(stationIDs)
	if err := r.q.DeleteUserStationsExcept(ctx, query.DeleteUserStationsExceptParams{UserID: uuid.UUID(userID), StationIds: uuids}); err != nil {
		return err
	}
	if len(uuids) == 0 {
		return nil
	}

	return r.q.CreateUserStations(ctx, query.CreateUserStationsParams{UserID: uuid.UUID(userID), StationIds: uuids})
}
