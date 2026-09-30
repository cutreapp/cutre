package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// GoodsBuilder はテスト用のgoods (グッズ) の行を組み立てる。
// 既定では、カテゴリー eventCategoryID の配下に並び順1の公開中のグッズを作る。
type GoodsBuilder struct {
	t               *testing.T
	db              queryRower
	eventCategoryID model.EventCategoryID
	name            string
	position        int32
	status          model.MasterStatus
	archiveMessage  *string
}

// NewGoodsBuilder はカテゴリー eventCategoryID の配下にグッズを作る GoodsBuilder を生成する。
func NewGoodsBuilder(t *testing.T, db queryRower, eventCategoryID model.EventCategoryID) *GoodsBuilder {
	t.Helper()

	return &GoodsBuilder{
		t:               t,
		db:              db,
		eventCategoryID: eventCategoryID,
		name:            "テストのグッズ",
		position:        1,
		status:          model.MasterStatusPublished,
	}
}

// WithName は名前を設定する。
func (b *GoodsBuilder) WithName(name string) *GoodsBuilder {
	b.name = name
	return b
}

// WithPosition は並び順を設定する。
func (b *GoodsBuilder) WithPosition(position int32) *GoodsBuilder {
	b.position = position
	return b
}

// WithArchived は理由 archiveMessage を残してアーカイブしたグッズにする。
func (b *GoodsBuilder) WithArchived(archiveMessage string) *GoodsBuilder {
	b.status = model.MasterStatusArchived
	b.archiveMessage = &archiveMessage
	return b
}

// WithDeleted は削除したグッズにする。
func (b *GoodsBuilder) WithDeleted() *GoodsBuilder {
	b.status = model.MasterStatusDeleted
	return b
}

// Build はグッズを挿入し、データベースが採番したIDを返す。
func (b *GoodsBuilder) Build() model.GoodsID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO goods (event_category_id, name, position, status, archive_message)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		uuid.UUID(b.eventCategoryID), b.name, b.position, string(b.status), b.archiveMessage,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のグッズの作成に失敗しました: %v", err)
	}

	return model.GoodsID(id)
}
