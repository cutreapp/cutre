package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
)

// PasswordResetTokenBuilder はテスト用のpassword_reset_tokensの行を組み立てる。
// トークンは常に既存のユーザーに属するため、所有するユーザーは必須で既定値を持たない。
type PasswordResetTokenBuilder struct {
	t         *testing.T
	db        queryRower
	userID    model.UserID
	token     string
	expiresAt time.Time
}

// NewPasswordResetTokenBuilder は PasswordResetTokenBuilder を生成する。
// 既定は期限内のトークンで、トークンは他と重複しない値にする。
func NewPasswordResetTokenBuilder(t *testing.T, db queryRower) *PasswordResetTokenBuilder {
	t.Helper()

	return &PasswordResetTokenBuilder{
		t:         t,
		db:        db,
		token:     "password-reset-" + uniqueToken(),
		expiresAt: model.PasswordResetTokenExpiresAt(time.Now()),
	}
}

// WithUserID は所有するユーザーを設定する。
func (b *PasswordResetTokenBuilder) WithUserID(userID model.UserID) *PasswordResetTokenBuilder {
	b.userID = userID
	return b
}

// WithExpiresAt は有効期限を設定する。期限切れのトークンを作るテストが使う。
func (b *PasswordResetTokenBuilder) WithExpiresAt(expiresAt time.Time) *PasswordResetTokenBuilder {
	b.expiresAt = expiresAt
	return b
}

// Token はリンクに載る平文のトークンを返す。
// 保存するのはダイジェストのため、リンクを組み立てるテストはここから平文を受け取る。
func (b *PasswordResetTokenBuilder) Token() string {
	return b.token
}

// Build はトークンを挿入し、データベースが採番したIDを返す。
func (b *PasswordResetTokenBuilder) Build() model.PasswordResetTokenID {
	b.t.Helper()

	if b.userID == (model.UserID{}) {
		b.t.Fatal("PasswordResetTokenBuilder にはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO password_reset_tokens (user_id, token_digest, expires_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		uuid.UUID(b.userID), auth.HashToken(b.token), b.expiresAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のパスワードリセットのトークンの作成に失敗しました: %v", err)
	}

	return model.PasswordResetTokenID(id)
}
