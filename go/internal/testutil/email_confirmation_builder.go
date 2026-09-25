package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// EmailConfirmationBuilder はテスト用のemail_confirmationsの行を組み立てる。
// 既定は未確認・誤入力0回・期限内で、コードが "123456" の確認。
type EmailConfirmationBuilder struct {
	t                   *testing.T
	db                  queryRower
	email               string
	code                string
	expiresAt           time.Time
	failedAttemptsCount int
	confirmedAt         *time.Time
}

// NewEmailConfirmationBuilder は EmailConfirmationBuilder を生成する。
func NewEmailConfirmationBuilder(t *testing.T, db queryRower) *EmailConfirmationBuilder {
	t.Helper()

	return &EmailConfirmationBuilder{
		t:         t,
		db:        db,
		email:     UniqueEmail("email-confirmation"),
		code:      "123456",
		expiresAt: model.EmailConfirmationExpiresAt(time.Now()),
	}
}

// WithEmail はメールアドレスを設定する。
func (b *EmailConfirmationBuilder) WithEmail(email string) *EmailConfirmationBuilder {
	b.email = email
	return b
}

// WithCode は確認コードを設定する。
func (b *EmailConfirmationBuilder) WithCode(code string) *EmailConfirmationBuilder {
	b.code = code
	return b
}

// WithExpiresAt は有効期限を設定する。期限切れの確認を作るテストが使う。
func (b *EmailConfirmationBuilder) WithExpiresAt(expiresAt time.Time) *EmailConfirmationBuilder {
	b.expiresAt = expiresAt
	return b
}

// WithFailedAttemptsCount は誤入力の回数を設定する。
func (b *EmailConfirmationBuilder) WithFailedAttemptsCount(count int) *EmailConfirmationBuilder {
	b.failedAttemptsCount = count
	return b
}

// WithConfirmedAt は指定した時刻に確認を済ませた確認にする。
func (b *EmailConfirmationBuilder) WithConfirmedAt(confirmedAt time.Time) *EmailConfirmationBuilder {
	b.confirmedAt = &confirmedAt
	return b
}

// Build は確認を挿入し、データベースが採番したIDを返す。
func (b *EmailConfirmationBuilder) Build() model.EmailConfirmationID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO email_confirmations (email, code, expires_at, failed_attempts_count, confirmed_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		b.email, b.code, b.expiresAt, b.failedAttemptsCount, b.confirmedAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のメールアドレスの確認の作成に失敗しました: %v", err)
	}

	return model.EmailConfirmationID(id)
}
