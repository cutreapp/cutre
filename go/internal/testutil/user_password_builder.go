package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
)

// DefaultBuilderPassword は UserPasswordBuilder が既定でハッシュ化する平文パスワード。
// 資格情報を用意するだけのテストが、この既知の値でログインできるようにする。
const DefaultBuilderPassword = "password123"

// UserPasswordBuilder はテスト用のuser_passwordsの行を組み立てる。
// パスワードは常に既存のユーザーに属するため、所有するユーザーは必須で既定値を持たない。
type UserPasswordBuilder struct {
	t        *testing.T
	db       queryRower
	userID   model.UserID
	password string
}

// NewUserPasswordBuilder は UserPasswordBuilder を生成する。
func NewUserPasswordBuilder(t *testing.T, db queryRower) *UserPasswordBuilder {
	t.Helper()

	return &UserPasswordBuilder{
		t:        t,
		db:       db,
		password: DefaultBuilderPassword,
	}
}

// WithUserID は所有するユーザーを設定する。
func (b *UserPasswordBuilder) WithUserID(userID model.UserID) *UserPasswordBuilder {
	b.userID = userID
	return b
}

// WithPassword はハッシュ化する平文パスワードを設定する。
func (b *UserPasswordBuilder) WithPassword(password string) *UserPasswordBuilder {
	b.password = password
	return b
}

// Build はパスワードをハッシュ化して資格情報を挿入し、データベースが採番したIDを返す。
func (b *UserPasswordBuilder) Build() model.UserPasswordID {
	b.t.Helper()

	if b.userID == (model.UserID{}) {
		b.t.Fatal("UserPasswordBuilder にはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	digest, err := auth.HashPassword(b.password)
	if err != nil {
		b.t.Fatalf("テスト用パスワードのハッシュ化に失敗しました: %v", err)
	}

	var id uuid.UUID
	err = b.db.QueryRowContext(context.Background(),
		`INSERT INTO user_passwords (user_id, password_digest)
		 VALUES ($1, $2)
		 RETURNING id`,
		uuid.UUID(b.userID), digest,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用パスワード資格情報の作成に失敗しました: %v", err)
	}

	return model.UserPasswordID(id)
}
