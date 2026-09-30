package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// UserStationBuilder はテスト用のuser_stations (ユーザーが交換場所に選んだ駅) の行を組み立てる。
type UserStationBuilder struct {
	t         *testing.T
	db        queryRower
	userID    model.UserID
	stationID model.StationID
}

// NewUserStationBuilder は、ユーザー userID が駅 stationID を交換場所に選んだ行を作る UserStationBuilder を生成する。
func NewUserStationBuilder(t *testing.T, db queryRower, userID model.UserID, stationID model.StationID) *UserStationBuilder {
	t.Helper()

	return &UserStationBuilder{t: t, db: db, userID: userID, stationID: stationID}
}

// Build は交換場所の駅を挿入する。
func (b *UserStationBuilder) Build() {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO user_stations (user_id, station_id) VALUES ($1, $2) RETURNING id`,
		uuid.UUID(b.userID), uuid.UUID(b.stationID),
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の交換場所の駅の作成に失敗しました: %v", err)
	}
}
