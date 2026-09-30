package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeMessageBuilder はテスト用のtrade_messages (交換のメッセージ) の行を組み立てる。
// 既定では、今の時刻に送った、取り消していないメッセージを作る。
type TradeMessageBuilder struct {
	t            *testing.T
	db           queryRower
	tradeID      model.TradeID
	senderUserID model.UserID
	body         string
	createdAt    time.Time
	retractedAt  *time.Time
}

// NewTradeMessageBuilder は、ユーザー senderUserID が交換 tradeID で本文 body を送ったメッセージを作る TradeMessageBuilder を生成する。
func NewTradeMessageBuilder(t *testing.T, db queryRower, tradeID model.TradeID, senderUserID model.UserID, body string) *TradeMessageBuilder {
	t.Helper()

	return &TradeMessageBuilder{t: t, db: db, tradeID: tradeID, senderUserID: senderUserID, body: body, createdAt: time.Now()}
}

// WithCreatedAt は送った時刻を設定する。出来事と並べる順を確かめるテストが使う。
func (b *TradeMessageBuilder) WithCreatedAt(createdAt time.Time) *TradeMessageBuilder {
	b.createdAt = createdAt
	return b
}

// WithRetractedAt は取り消した時刻を設定する。設定すると取り消したメッセージになる。
func (b *TradeMessageBuilder) WithRetractedAt(retractedAt time.Time) *TradeMessageBuilder {
	b.retractedAt = &retractedAt
	return b
}

// Build はメッセージを挿入し、データベースが採番したIDを返す。
func (b *TradeMessageBuilder) Build() model.TradeMessageID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO trade_messages (trade_id, sender_user_id, body, retracted_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id`,
		uuid.UUID(b.tradeID), uuid.UUID(b.senderUserID), b.body, b.retractedAt, b.createdAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の交換のメッセージの作成に失敗しました: %v", err)
	}

	return model.TradeMessageID(id)
}
