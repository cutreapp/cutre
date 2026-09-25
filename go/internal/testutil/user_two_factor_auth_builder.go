package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// UserTwoFactorAuthBuilder はテスト用のuser_two_factor_authsの行を組み立てる。
// 既定では、復号できない仮の暗号文を持つ、認証アプリへの登録の途中の設定を作る。
type UserTwoFactorAuthBuilder struct {
	t                *testing.T
	db               queryRower
	userID           model.UserID
	secretCiphertext []byte
	enabledAt        *time.Time
}

// NewUserTwoFactorAuthBuilder は userID のユーザーの UserTwoFactorAuthBuilder を生成する。
func NewUserTwoFactorAuthBuilder(t *testing.T, db queryRower, userID model.UserID) *UserTwoFactorAuthBuilder {
	t.Helper()

	return &UserTwoFactorAuthBuilder{t: t, db: db, userID: userID, secretCiphertext: []byte("test-secret-ciphertext")}
}

// WithSecretCiphertext は暗号化したTOTPの秘密鍵を設定する。コードを照合するテストが、実際に暗号化した値を渡す。
func (b *UserTwoFactorAuthBuilder) WithSecretCiphertext(secretCiphertext []byte) *UserTwoFactorAuthBuilder {
	b.secretCiphertext = secretCiphertext
	return b
}

// WithEnabledAt は二要素認証を有効にした時刻を設定する。設定すると有効な設定になる。
func (b *UserTwoFactorAuthBuilder) WithEnabledAt(enabledAt time.Time) *UserTwoFactorAuthBuilder {
	b.enabledAt = &enabledAt
	return b
}

// Build は二要素認証の設定を挿入し、データベースが採番したIDを返す。
func (b *UserTwoFactorAuthBuilder) Build() model.UserTwoFactorAuthID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO user_two_factor_auths (user_id, secret_ciphertext, enabled_at)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		uuid.UUID(b.userID), b.secretCiphertext, b.enabledAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用の二要素認証の設定の作成に失敗しました: %v", err)
	}

	return model.UserTwoFactorAuthID(id)
}
