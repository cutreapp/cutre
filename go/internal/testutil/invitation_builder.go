package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// InvitationBuilder はテスト用のinvitationsの行を組み立てる。
// 既定は管理者が発行した (招待者の無い)、未取り消し・期限内の招待。
// 使用の記録は InvitationRedemptionBuilder で足す。
type InvitationBuilder struct {
	t             *testing.T
	db            queryRower
	inviterUserID *model.UserID
	token         string
	expiresAt     time.Time
	revokedAt     *time.Time
}

// NewInvitationBuilder は InvitationBuilder を生成する。
// 既定のトークンは他と重複しない値のため、複数の招待を作るテストが値を1つずつ決める必要はない。
func NewInvitationBuilder(t *testing.T, db queryRower) *InvitationBuilder {
	t.Helper()

	return &InvitationBuilder{
		t:         t,
		db:        db,
		token:     "invitation-" + uniqueToken(),
		expiresAt: model.InvitationExpiresAt(time.Now()),
	}
}

// WithInviterUserID は招待したユーザーを設定する。
func (b *InvitationBuilder) WithInviterUserID(userID model.UserID) *InvitationBuilder {
	b.inviterUserID = &userID
	return b
}

// WithExpiresAt は有効期限を設定する。期限切れの招待を作るテストが使う。
func (b *InvitationBuilder) WithExpiresAt(expiresAt time.Time) *InvitationBuilder {
	b.expiresAt = expiresAt
	return b
}

// WithRevokedAt は指定した時刻に取り消した招待にする。
func (b *InvitationBuilder) WithRevokedAt(revokedAt time.Time) *InvitationBuilder {
	b.revokedAt = &revokedAt
	return b
}

// Token は招待リンクに載るトークンを返す。
func (b *InvitationBuilder) Token() string {
	return b.token
}

// Build は招待を挿入し、データベースが採番したIDを返す。
func (b *InvitationBuilder) Build() model.InvitationID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO invitations (inviter_user_id, token, expires_at, revoked_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		userIDOrNil(b.inviterUserID), b.token, b.expiresAt, b.revokedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の招待の作成に失敗しました: %v", err)
	}

	return model.InvitationID(id)
}

// userIDOrNil はNULL許容のユーザーIDを、SQLの引数に渡せる値へ変換する。
func userIDOrNil(id *model.UserID) any {
	if id == nil {
		return nil
	}

	return uuid.UUID(*id)
}
