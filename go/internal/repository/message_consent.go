package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// MessageConsentRepository はmessage_consentsを読み書きする。
type MessageConsentRepository struct {
	q *query.Queries
}

// NewMessageConsentRepository は MessageConsentRepository を生成する。
func NewMessageConsentRepository(db *sql.DB) *MessageConsentRepository {
	return &MessageConsentRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい MessageConsentRepository を返す。
// アカウントの作成と同じトランザクションで同意を記録するUseCaseがこれを使う。
func (r *MessageConsentRepository) WithTx(tx *sql.Tx) *MessageConsentRepository {
	return &MessageConsentRepository{q: r.q.WithTx(tx)}
}

// Create は、ユーザーが version の版の文面に今の時刻で同意したことを記録する。
// 版は列の型に合わせてint32で受ける。呼び出し側は model.CurrentMessageConsentVersion をそのまま渡せる。
func (r *MessageConsentRepository) Create(ctx context.Context, userID model.UserID, version int32) (*model.MessageConsent, error) {
	row, err := r.q.CreateMessageConsent(ctx, query.CreateMessageConsentParams{
		UserID:  uuid.UUID(userID),
		Version: version,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// FindLatestByUserID はユーザーの最新の同意の記録を返す。一度も同意していないときは (nil, nil) を返す。
// やめた記録や古い版の記録も返すため、有効かどうかは呼び出し側が IsValid で確かめる。
func (r *MessageConsentRepository) FindLatestByUserID(ctx context.Context, userID model.UserID) (*model.MessageConsent, error) {
	row, err := r.q.GetLatestMessageConsentByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// WithdrawByUserID は、ユーザーのやめていない同意をすべてやめたことにする。
// やめていない同意が無かったときはfalseを返す。
func (r *MessageConsentRepository) WithdrawByUserID(ctx context.Context, userID model.UserID) (bool, error) {
	affected, err := r.q.WithdrawMessageConsentsByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// toModel はクエリの行を model.MessageConsent に変換する。
func (r *MessageConsentRepository) toModel(row query.MessageConsent) *model.MessageConsent {
	return &model.MessageConsent{
		ID:          model.MessageConsentID(row.ID),
		UserID:      model.UserID(row.UserID),
		Version:     int(row.Version),
		AgreedAt:    row.AgreedAt,
		WithdrawnAt: row.WithdrawnAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
