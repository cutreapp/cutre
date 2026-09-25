package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// EmailConfirmationRepository はemail_confirmationsを読み書きする。
type EmailConfirmationRepository struct {
	q *query.Queries
}

// NewEmailConfirmationRepository は EmailConfirmationRepository を生成する。
func NewEmailConfirmationRepository(db *sql.DB) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい EmailConfirmationRepository を返す。
// 確認コードの保存とメールの送信のジョブの投入を1つのトランザクションで行うUseCaseがこれを使う。
func (r *EmailConfirmationRepository) WithTx(tx *sql.Tx) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{q: r.q.WithTx(tx)}
}

// CreateEmailConfirmationInput は確認の作成に必要な属性。
// idとタイムスタンプ (created_at・updated_at) はデータベースが採番する。
type CreateEmailConfirmationInput struct {
	Email     string
	Code      string
	ExpiresAt time.Time
}

// Create は確認を挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *EmailConfirmationRepository) Create(ctx context.Context, input CreateEmailConfirmationInput) (*model.EmailConfirmation, error) {
	row, err := r.q.CreateEmailConfirmation(ctx, query.CreateEmailConfirmationParams{
		Email:     input.Email,
		Code:      input.Code,
		ExpiresAt: input.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// FindUnconfirmedByID は確認済みでない確認を返す。無い・確認済みのときはnilを返す。
// 有効期限と誤入力の回数では絞らない。コードの再送は、期限切れや上限に達した確認からも行えるようにするため。
func (r *EmailConfirmationRepository) FindUnconfirmedByID(ctx context.Context, id model.EmailConfirmationID) (*model.EmailConfirmation, error) {
	row, err := r.q.GetUnconfirmedEmailConfirmationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// FindConfirmedByID は確認済みの確認を返す。無い・未確認のときはnilを返す。
func (r *EmailConfirmationRepository) FindConfirmedByID(ctx context.Context, id model.EmailConfirmationID) (*model.EmailConfirmation, error) {
	row, err := r.q.GetConfirmedEmailConfirmationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// DeleteConfirmed は確認済みの確認を削除する。無い・未確認で削除しなかったときはfalseを返す。
func (r *EmailConfirmationRepository) DeleteConfirmed(ctx context.Context, id model.EmailConfirmationID) (bool, error) {
	affected, err := r.q.DeleteConfirmedEmailConfirmation(ctx, uuid.UUID(id))
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteExpired はbeforeまでに有効期限が切れた確認を、確認済みかどうかによらず削除する。
func (r *EmailConfirmationRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return r.q.DeleteExpiredEmailConfirmations(ctx, before)
}

// DeleteByEmail は指定したメールアドレスへの確認を、確認済みかどうかによらず削除する。
// email列はcitextのため、照合は大文字小文字を無視する。
func (r *EmailConfirmationRepository) DeleteByEmail(ctx context.Context, email string) error {
	return r.q.DeleteEmailConfirmationsByEmail(ctx, email)
}

// Verify は入力されたコードを照合し、一致すれば確認済みにし、違えば誤入力の回数を増やした確認を返す。
// 期限切れ・確認済み・誤入力の回数が上限に達した確認は照合せず、nilを返す。
func (r *EmailConfirmationRepository) Verify(ctx context.Context, id model.EmailConfirmationID, code string) (*model.EmailConfirmation, error) {
	row, err := r.q.VerifyEmailConfirmation(ctx, query.VerifyEmailConfirmationParams{
		ID:                uuid.UUID(id),
		Code:              code,
		MaxFailedAttempts: model.EmailConfirmationMaxFailedAttempts,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// toModel はクエリの行を model.EmailConfirmation に変換する。
func (r *EmailConfirmationRepository) toModel(row query.EmailConfirmation) *model.EmailConfirmation {
	return &model.EmailConfirmation{
		ID:                  model.EmailConfirmationID(row.ID),
		Email:               row.Email,
		Code:                row.Code,
		ExpiresAt:           row.ExpiresAt,
		FailedAttemptsCount: row.FailedAttemptsCount,
		ConfirmedAt:         row.ConfirmedAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}
