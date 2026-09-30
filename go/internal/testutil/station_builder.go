package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// StationBuilder はテスト用のstations (駅) の行を組み立てる。
// 既定では、東京都 (13) に並び順1の公開中の駅を作る。
type StationBuilder struct {
	t              *testing.T
	db             queryRower
	prefectureCode model.PrefectureCode
	name           string
	position       int32
	status         model.MasterStatus
	archiveMessage *string
}

// NewStationBuilder は StationBuilder を生成する。
func NewStationBuilder(t *testing.T, db queryRower) *StationBuilder {
	t.Helper()

	return &StationBuilder{
		t:              t,
		db:             db,
		prefectureCode: 13,
		name:           "テストの駅",
		position:       1,
		status:         model.MasterStatusPublished,
	}
}

// WithPrefectureCode は都道府県コードを設定する。
func (b *StationBuilder) WithPrefectureCode(code model.PrefectureCode) *StationBuilder {
	b.prefectureCode = code
	return b
}

// WithName は名前を設定する。
func (b *StationBuilder) WithName(name string) *StationBuilder {
	b.name = name
	return b
}

// WithPosition は並び順を設定する。
func (b *StationBuilder) WithPosition(position int32) *StationBuilder {
	b.position = position
	return b
}

// WithArchived は理由 archiveMessage を残してアーカイブした駅にする。
func (b *StationBuilder) WithArchived(archiveMessage string) *StationBuilder {
	b.status = model.MasterStatusArchived
	b.archiveMessage = &archiveMessage
	return b
}

// WithDeleted は削除した駅にする。
func (b *StationBuilder) WithDeleted() *StationBuilder {
	b.status = model.MasterStatusDeleted
	return b
}

// Build は駅を挿入し、データベースが採番したIDを返す。
func (b *StationBuilder) Build() model.StationID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO stations (prefecture_code, name, position, status, archive_message)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		int16(b.prefectureCode), b.name, b.position, string(b.status), b.archiveMessage,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の駅の作成に失敗しました: %v", err)
	}

	return model.StationID(id)
}
