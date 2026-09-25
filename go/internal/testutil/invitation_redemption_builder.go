package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// InvitationRedemptionBuilder はテスト用のinvitation_redemptionsの行を組み立てる。
// 既定では、招待で登録したユーザーを新しく作る。
type InvitationRedemptionBuilder struct {
	t            *testing.T
	db           queryRower
	invitationID model.InvitationID
	userID       *model.UserID
	createdAt    *time.Time
}

// NewInvitationRedemptionBuilder は invitationID の招待を使った記録の InvitationRedemptionBuilder を生成する。
func NewInvitationRedemptionBuilder(t *testing.T, db queryRower, invitationID model.InvitationID) *InvitationRedemptionBuilder {
	t.Helper()

	return &InvitationRedemptionBuilder{t: t, db: db, invitationID: invitationID}
}

// WithUserID は招待で登録したユーザーを設定する。
func (b *InvitationRedemptionBuilder) WithUserID(userID model.UserID) *InvitationRedemptionBuilder {
	b.userID = &userID
	return b
}

// WithCreatedAt は招待を使った (登録した) 時刻を設定する。参加日の表示や並び順を確かめるテストが使う。
func (b *InvitationRedemptionBuilder) WithCreatedAt(createdAt time.Time) *InvitationRedemptionBuilder {
	b.createdAt = &createdAt
	return b
}

// Build は使用の記録を挿入し、データベースが採番したIDを返す。
func (b *InvitationRedemptionBuilder) Build() model.InvitationRedemptionID {
	b.t.Helper()

	userID := b.userID
	if userID == nil {
		id := NewUserBuilder(b.t, b.db).Build()
		userID = &id
	}

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO invitation_redemptions (invitation_id, user_id, created_at, updated_at)
		 VALUES ($1, $2, COALESCE($3, NOW()), COALESCE($3, NOW()))
		 RETURNING id`,
		uuid.UUID(b.invitationID), uuid.UUID(*userID), b.createdAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の招待の使用の記録の作成に失敗しました: %v", err)
	}

	return model.InvitationRedemptionID(id)
}
