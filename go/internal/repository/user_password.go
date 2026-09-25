package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// UserPasswordRepository はuser_passwordsを読み書きする。
type UserPasswordRepository struct {
	q *query.Queries
}

// NewUserPasswordRepository は UserPasswordRepository を生成する。
func NewUserPasswordRepository(db *sql.DB) *UserPasswordRepository {
	return &UserPasswordRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserPasswordRepository を返す。
// ユーザーとパスワードを1つのトランザクションで作るUseCaseや、
// パスワードの置き換えとトークン・セッションの削除を1つのトランザクションで行うUseCaseがこれを使う。
func (r *UserPasswordRepository) WithTx(tx *sql.Tx) *UserPasswordRepository {
	return &UserPasswordRepository{q: r.q.WithTx(tx)}
}

// FindByUserID は指定したユーザーのパスワード資格情報を返す。
// 存在しない場合 (アカウントの作成が完了していないなど) は (nil, nil) を返す。
func (r *UserPasswordRepository) FindByUserID(ctx context.Context, userID model.UserID) (*model.UserPassword, error) {
	row, err := r.q.GetUserPasswordByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// CreateUserPasswordInput はパスワード資格情報の作成に必要な属性。
// PasswordDigest は既にbcryptハッシュである必要があり、このリポジトリは平文を扱わない。
type CreateUserPasswordInput struct {
	UserID         model.UserID
	PasswordDigest string
}

// Create はパスワード資格情報を挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *UserPasswordRepository) Create(ctx context.Context, input CreateUserPasswordInput) (*model.UserPassword, error) {
	row, err := r.q.CreateUserPassword(ctx, query.CreateUserPasswordParams{
		UserID:         uuid.UUID(input.UserID),
		PasswordDigest: input.PasswordDigest,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// UpdatePasswordDigest は指定したユーザーのパスワードのダイジェストを置き換える。
// パスワード資格情報が無く、置き換えなかったときはfalseを返す。
// passwordDigest は既にbcryptハッシュである必要がある。
func (r *UserPasswordRepository) UpdatePasswordDigest(ctx context.Context, userID model.UserID, passwordDigest string) (bool, error) {
	affected, err := r.q.UpdateUserPasswordDigest(ctx, query.UpdateUserPasswordDigestParams{
		UserID:         uuid.UUID(userID),
		PasswordDigest: passwordDigest,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteByUserID は指定したユーザーのパスワード資格情報を削除する。無いときも失敗にしない。
func (r *UserPasswordRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeleteUserPasswordByUserID(ctx, uuid.UUID(userID))
}

// toModel はクエリの行を model.UserPassword に変換する。
func (r *UserPasswordRepository) toModel(row query.UserPassword) *model.UserPassword {
	return &model.UserPassword{
		ID:             model.UserPasswordID(row.ID),
		UserID:         model.UserID(row.UserID),
		PasswordDigest: row.PasswordDigest,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
