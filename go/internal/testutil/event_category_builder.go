package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// EventCategoryBuilder はテスト用のevent_categories (カテゴリー) の行を組み立てる。
// 既定では、イベント eventID の配下に並び順1の公開中のカテゴリーを作る。
type EventCategoryBuilder struct {
	t              *testing.T
	db             queryRower
	eventID        model.EventID
	name           string
	position       int32
	status         model.MasterStatus
	archiveMessage *string
}

// NewEventCategoryBuilder はイベント eventID の配下にカテゴリーを作る EventCategoryBuilder を生成する。
func NewEventCategoryBuilder(t *testing.T, db queryRower, eventID model.EventID) *EventCategoryBuilder {
	t.Helper()

	return &EventCategoryBuilder{
		t:        t,
		db:       db,
		eventID:  eventID,
		name:     "テストのカテゴリー",
		position: 1,
		status:   model.MasterStatusPublished,
	}
}

// WithName は名前を設定する。
func (b *EventCategoryBuilder) WithName(name string) *EventCategoryBuilder {
	b.name = name
	return b
}

// WithPosition は並び順を設定する。
func (b *EventCategoryBuilder) WithPosition(position int32) *EventCategoryBuilder {
	b.position = position
	return b
}

// WithArchived は理由 archiveMessage を残してアーカイブしたカテゴリーにする。
func (b *EventCategoryBuilder) WithArchived(archiveMessage string) *EventCategoryBuilder {
	b.status = model.MasterStatusArchived
	b.archiveMessage = &archiveMessage
	return b
}

// WithDeleted は削除したカテゴリーにする。
func (b *EventCategoryBuilder) WithDeleted() *EventCategoryBuilder {
	b.status = model.MasterStatusDeleted
	return b
}

// Build はカテゴリーを挿入し、データベースが採番したIDを返す。
func (b *EventCategoryBuilder) Build() model.EventCategoryID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO event_categories (event_id, name, position, status, archive_message)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		uuid.UUID(b.eventID), b.name, b.position, string(b.status), b.archiveMessage,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のカテゴリーの作成に失敗しました: %v", err)
	}

	return model.EventCategoryID(id)
}
