package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
)

// UserSessionBuilder はテスト用のuser_sessionsの行を組み立てる。
// セッションは常に既存のユーザーに属するため、所有するユーザーは必須で既定値を持たない。
type UserSessionBuilder struct {
	t          *testing.T
	db         queryRower
	userID     model.UserID
	token      string
	expiresAt  time.Time
	lastSeenAt time.Time
	ipAddress  string
	userAgent  string
}

// NewUserSessionBuilder は UserSessionBuilder を生成する。
// 既定のトークンは他と重複しない値のため、複数のセッションを作るテストが値を1つずつ決める必要はない。
func NewUserSessionBuilder(t *testing.T, db queryRower) *UserSessionBuilder {
	t.Helper()

	now := time.Now()

	return &UserSessionBuilder{
		t:          t,
		db:         db,
		token:      "token-" + uniqueToken(),
		expiresAt:  model.UserSessionExpiresAt(now),
		lastSeenAt: now,
		ipAddress:  "192.0.2.1",
		userAgent:  "cutre-test",
	}
}

// WithUserID は所有するユーザーを設定する。
func (b *UserSessionBuilder) WithUserID(userID model.UserID) *UserSessionBuilder {
	b.userID = userID
	return b
}

// WithToken はCookieに入る平文のトークンを設定する。
func (b *UserSessionBuilder) WithToken(token string) *UserSessionBuilder {
	b.token = token
	return b
}

// WithExpiresAt は有効期限を設定する。期限切れのセッションを作るテストが使う。
func (b *UserSessionBuilder) WithExpiresAt(expiresAt time.Time) *UserSessionBuilder {
	b.expiresAt = expiresAt
	return b
}

// WithLastSeenAt は最後に使った時刻を設定する。期限の延長を確かめるテストが使う。
func (b *UserSessionBuilder) WithLastSeenAt(lastSeenAt time.Time) *UserSessionBuilder {
	b.lastSeenAt = lastSeenAt
	return b
}

// Token はこのセッションのCookieに入る平文のトークンを返す。
// 保存するのはダイジェストのため、リクエストを組み立てるテストはここから平文を受け取る。
func (b *UserSessionBuilder) Token() string {
	return b.token
}

// Build はセッションを挿入し、データベースが採番したIDを返す。
func (b *UserSessionBuilder) Build() model.UserSessionID {
	b.t.Helper()

	if b.userID == (model.UserID{}) {
		b.t.Fatal("UserSessionBuilder にはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO user_sessions (user_id, token_digest, expires_at, last_seen_at, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		uuid.UUID(b.userID), auth.HashToken(b.token), b.expiresAt, b.lastSeenAt, b.ipAddress, b.userAgent,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用セッションの作成に失敗しました: %v", err)
	}

	return model.UserSessionID(id)
}
