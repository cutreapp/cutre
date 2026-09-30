package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
)

// MessageConsentBuilder はテスト用のmessage_consentsの行を組み立てる。
// 既定では、今の版の文面に今の時刻で同意し、やめていない記録を作る。
type MessageConsentBuilder struct {
	t           *testing.T
	db          queryRower
	userID      model.UserID
	version     int
	agreedAt    time.Time
	withdrawnAt *time.Time
}

// NewMessageConsentBuilder は userID のユーザーの MessageConsentBuilder を生成する。
func NewMessageConsentBuilder(t *testing.T, db queryRower, userID model.UserID) *MessageConsentBuilder {
	t.Helper()

	return &MessageConsentBuilder{t: t, db: db, userID: userID, version: model.CurrentMessageConsentVersion, agreedAt: time.Now()}
}

// WithVersion は同意した文面の版を設定する。古い版の同意を作るテストが使う。
func (b *MessageConsentBuilder) WithVersion(version int) *MessageConsentBuilder {
	b.version = version
	return b
}

// WithAgreedAt は同意した時刻を設定する。
func (b *MessageConsentBuilder) WithAgreedAt(agreedAt time.Time) *MessageConsentBuilder {
	b.agreedAt = agreedAt
	return b
}

// WithWithdrawnAt は同意をやめた時刻を設定する。設定するとやめた記録になる。
func (b *MessageConsentBuilder) WithWithdrawnAt(withdrawnAt time.Time) *MessageConsentBuilder {
	b.withdrawnAt = &withdrawnAt
	return b
}

// Build は同意の記録を挿入し、データベースが採番したIDを返す。
func (b *MessageConsentBuilder) Build() model.MessageConsentID {
	b.t.Helper()

	var id uuid.UUID
	err := b.db.QueryRowContext(context.Background(),
		`INSERT INTO message_consents (user_id, version, agreed_at, withdrawn_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		uuid.UUID(b.userID), b.version, b.agreedAt, b.withdrawnAt,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用のメッセージの取り扱いへの同意の作成に失敗しました: %v", err)
	}

	return model.MessageConsentID(id)
}
