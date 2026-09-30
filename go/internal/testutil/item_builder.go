package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// ItemBuilder はテスト用のitems (リストのアイテム) の行を組み立てる。
// 既定では、ユーザー userID の譲れるリストに、グッズ goodsID を1点入れたアイテムを作る。
type ItemBuilder struct {
	t        *testing.T
	db       queryRower
	userID   model.UserID
	goodsID  model.GoodsID
	kind     model.ItemKind
	status   model.ItemStatus
	quantity int32
	note     string
}

// NewItemBuilder はユーザー userID のグッズ goodsID のアイテムを作る ItemBuilder を生成する。
func NewItemBuilder(t *testing.T, db queryRower, userID model.UserID, goodsID model.GoodsID) *ItemBuilder {
	t.Helper()

	return &ItemBuilder{
		t:        t,
		db:       db,
		userID:   userID,
		goodsID:  goodsID,
		kind:     model.ItemKindGive,
		status:   model.ItemStatusListed,
		quantity: 1,
	}
}

// WithKind はアイテムを入れるリストを設定する。
func (b *ItemBuilder) WithKind(kind model.ItemKind) *ItemBuilder {
	b.kind = kind
	return b
}

// WithQuantity は数量を設定する。
func (b *ItemBuilder) WithQuantity(quantity int32) *ItemBuilder {
	b.quantity = quantity
	return b
}

// WithNote はひとことを設定する。
func (b *ItemBuilder) WithNote(note string) *ItemBuilder {
	b.note = note
	return b
}

// WithRemoved はリストから外したアイテムにする。
func (b *ItemBuilder) WithRemoved() *ItemBuilder {
	b.status = model.ItemStatusRemoved
	return b
}

// Build はアイテムを挿入し、データベースが採番したIDを返す。
func (b *ItemBuilder) Build() model.ItemID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO items (user_id, goods_id, kind, status, quantity, note)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		uuid.UUID(b.userID), uuid.UUID(b.goodsID), string(b.kind), string(b.status), b.quantity, b.note,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のアイテムの作成に失敗しました: %v", err)
	}

	return model.ItemID(id)
}
