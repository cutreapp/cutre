package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// EventBuilder はテスト用のeventsの行を組み立てる。
// 既定では、終わりの決まっていない公開中のイベントを作る。
type EventBuilder struct {
	t              *testing.T
	db             queryRower
	name           string
	startsOn       time.Time
	endsOn         *time.Time
	status         model.MasterStatus
	archiveMessage *string
}

// NewEventBuilder は EventBuilder を生成する。
func NewEventBuilder(t *testing.T, db queryRower) *EventBuilder {
	t.Helper()

	return &EventBuilder{
		t:        t,
		db:       db,
		name:     "テストのイベント",
		startsOn: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		status:   model.MasterStatusPublished,
	}
}

// WithName は名前を設定する。
func (b *EventBuilder) WithName(name string) *EventBuilder {
	b.name = name
	return b
}

// WithPeriod は開催期間を設定する。endsOn がnilなら終わりの決まっていないイベントにする。
func (b *EventBuilder) WithPeriod(startsOn time.Time, endsOn *time.Time) *EventBuilder {
	b.startsOn = startsOn
	b.endsOn = endsOn
	return b
}

// WithArchived は理由 archiveMessage を残してアーカイブしたイベントにする。
func (b *EventBuilder) WithArchived(archiveMessage string) *EventBuilder {
	b.status = model.MasterStatusArchived
	b.archiveMessage = &archiveMessage
	return b
}

// WithDeleted は削除したイベントにする。
func (b *EventBuilder) WithDeleted() *EventBuilder {
	b.status = model.MasterStatusDeleted
	return b
}

// Build はイベントを挿入し、データベースが採番したIDを返す。
func (b *EventBuilder) Build() model.EventID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO events (name, starts_on, ends_on, status, archive_message)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		b.name, b.startsOn, b.endsOn, string(b.status), b.archiveMessage,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のイベントの作成に失敗しました: %v", err)
	}

	return model.EventID(id)
}
