package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// TradeBuilder はテスト用のtrades (交換) の行を組み立てる。
// 既定では、ユーザー proposerUserID がユーザー receiverUserID に申し込んだ、返事待ちの交換を作る。
// 終わった段階の交換は、終わった時刻を指定しなければ作った時刻で終わったものにする。
type TradeBuilder struct {
	t              *testing.T
	db             queryRower
	proposerUserID model.UserID
	receiverUserID model.UserID
	status         model.TradeStatus
	// proposerCompletedAt・receiverCompletedAt は、それぞれが「交換できた」を押した時刻。nilなら押していない。
	proposerCompletedAt *time.Time
	receiverCompletedAt *time.Time
	// endedAt は交換が終わった時刻。nilなら、終わった段階のときに作った時刻を入れる。
	endedAt *time.Time
}

// NewTradeBuilder は proposerUserID が receiverUserID に申し込んだ交換を作る TradeBuilder を生成する。
func NewTradeBuilder(t *testing.T, db queryRower, proposerUserID, receiverUserID model.UserID) *TradeBuilder {
	t.Helper()

	return &TradeBuilder{t: t, db: db, proposerUserID: proposerUserID, receiverUserID: receiverUserID, status: model.TradeStatusPending}
}

// WithStatus は交換の段階を設定する。
func (b *TradeBuilder) WithStatus(status model.TradeStatus) *TradeBuilder {
	b.status = status
	return b
}

// WithProposerCompletedAt は、申し込んだ人が「交換できた」を押した時刻を設定する。
func (b *TradeBuilder) WithProposerCompletedAt(at time.Time) *TradeBuilder {
	b.proposerCompletedAt = &at
	return b
}

// WithReceiverCompletedAt は、申し込まれた人が「交換できた」を押した時刻を設定する。
func (b *TradeBuilder) WithReceiverCompletedAt(at time.Time) *TradeBuilder {
	b.receiverCompletedAt = &at
	return b
}

// WithEndedAt は交換が終わった時刻を設定する。
func (b *TradeBuilder) WithEndedAt(at time.Time) *TradeBuilder {
	b.endedAt = &at
	return b
}

// Build は交換を挿入し、データベースが採番したIDを返す。
func (b *TradeBuilder) Build() model.TradeID {
	b.t.Helper()

	endedAt := b.endedAt
	if endedAt == nil && b.status != model.TradeStatusPending && b.status != model.TradeStatusMatched {
		now := time.Now()
		endedAt = &now
	}

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO trades (proposer_user_id, receiver_user_id, status, proposer_completed_at, receiver_completed_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		uuid.UUID(b.proposerUserID), uuid.UUID(b.receiverUserID), string(b.status), b.proposerCompletedAt, b.receiverCompletedAt, endedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の交換の作成に失敗しました: %v", err)
	}

	return model.TradeID(id)
}
