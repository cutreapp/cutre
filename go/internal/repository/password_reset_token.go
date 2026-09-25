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

// PasswordResetTokenRepository はpassword_reset_tokensを読み書きする。
type PasswordResetTokenRepository struct {
	q *query.Queries
}

// NewPasswordResetTokenRepository は PasswordResetTokenRepository を生成する。
func NewPasswordResetTokenRepository(db *sql.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい PasswordResetTokenRepository を返す。
// 古いトークンの削除と新しいトークンの作成を1つのトランザクションで行うUseCaseや、
// トークンの使用とパスワードの置き換えを1つのトランザクションで行うUseCaseがこれを使う。
func (r *PasswordResetTokenRepository) WithTx(tx *sql.Tx) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{q: r.q.WithTx(tx)}
}

// CreatePasswordResetTokenInput はトークンの作成に必要な属性。
// idとタイムスタンプ (created_at・updated_at) はデータベースが採番する。
//
// TokenDigest は既にダイジェストである必要があり、このリポジトリは平文のトークンを扱わない。
type CreatePasswordResetTokenInput struct {
	UserID      model.UserID
	TokenDigest string
	ExpiresAt   time.Time
}

// Create はトークンを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *PasswordResetTokenRepository) Create(ctx context.Context, input CreatePasswordResetTokenInput) (*model.PasswordResetToken, error) {
	row, err := r.q.CreatePasswordResetToken(ctx, query.CreatePasswordResetTokenParams{
		UserID:      uuid.UUID(input.UserID),
		TokenDigest: input.TokenDigest,
		ExpiresAt:   input.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// DeleteByUserID は指定したユーザーのトークンをすべて削除する。
// 新しいトークンを発行する前に呼び、以前に送ったリンクを使えなくする。
func (r *PasswordResetTokenRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeletePasswordResetTokensByUserID(ctx, uuid.UUID(userID))
}

// FindLiveByTokenDigest はダイジェストが一致する期限内のトークンを返す。
// 無い・期限切れのときは (nil, nil) を返す。
func (r *PasswordResetTokenRepository) FindLiveByTokenDigest(ctx context.Context, tokenDigest string) (*model.PasswordResetToken, error) {
	return r.findLive(r.q.GetLivePasswordResetTokenByTokenDigest(ctx, tokenDigest))
}

// FindLiveByID は期限内のトークンをIDで返す。無い・期限切れのときは (nil, nil) を返す。
func (r *PasswordResetTokenRepository) FindLiveByID(ctx context.Context, id model.PasswordResetTokenID) (*model.PasswordResetToken, error) {
	return r.findLive(r.q.GetLivePasswordResetTokenByID(ctx, uuid.UUID(id)))
}

// DeleteLive は期限内のトークンを削除する。無い・期限切れで削除しなかったときはfalseを返す。
// トークンを使うときに呼び、同じトークンで2度パスワードを変えられないようにする。
func (r *PasswordResetTokenRepository) DeleteLive(ctx context.Context, id model.PasswordResetTokenID) (bool, error) {
	affected, err := r.q.DeleteLivePasswordResetToken(ctx, uuid.UUID(id))
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// findLive は1行を引くクエリの結果をモデルに変換する。行が無いときは (nil, nil) を返す。
func (r *PasswordResetTokenRepository) findLive(row query.PasswordResetToken, err error) (*model.PasswordResetToken, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// toModel はクエリの行を model.PasswordResetToken に変換する。
func (r *PasswordResetTokenRepository) toModel(row query.PasswordResetToken) *model.PasswordResetToken {
	return &model.PasswordResetToken{
		ID:          model.PasswordResetTokenID(row.ID),
		UserID:      model.UserID(row.UserID),
		TokenDigest: row.TokenDigest,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
