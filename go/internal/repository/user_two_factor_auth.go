package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// UserTwoFactorAuthRepository はuser_two_factor_authsを読み書きする。
//
// 秘密鍵は暗号化した値のまま受け渡し、このリポジトリは平文の秘密鍵を扱わない。
type UserTwoFactorAuthRepository struct {
	q *query.Queries
}

// NewUserTwoFactorAuthRepository は UserTwoFactorAuthRepository を生成する。
func NewUserTwoFactorAuthRepository(db *sql.DB) *UserTwoFactorAuthRepository {
	return &UserTwoFactorAuthRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserTwoFactorAuthRepository を返す。
// 有効化とリカバリーコードの作成、無効化とリカバリーコードの削除を1つのトランザクションで行うUseCaseがこれを使う。
func (r *UserTwoFactorAuthRepository) WithTx(tx *sql.Tx) *UserTwoFactorAuthRepository {
	return &UserTwoFactorAuthRepository{q: r.q.WithTx(tx)}
}

// UpsertPending は認証アプリへの登録の途中の設定を作るか、その秘密鍵を差し替える。
// 既に有効にしているときは書き換えず (nil, nil) を返す。
//
// 登録の画面を開き直すたびに新しい秘密鍵にするため、途中の設定は上書きしてよい。
// 有効な設定を上書きすると、登録済みの認証アプリのコードが通らなくなるため守る。
func (r *UserTwoFactorAuthRepository) UpsertPending(ctx context.Context, userID model.UserID, secretCiphertext []byte) (*model.UserTwoFactorAuth, error) {
	row, err := r.q.UpsertPendingUserTwoFactorAuth(ctx, query.UpsertPendingUserTwoFactorAuthParams{
		UserID:           uuid.UUID(userID),
		SecretCiphertext: secretCiphertext,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// FindByUserID はユーザーの二要素認証の設定を返す。無いときは (nil, nil) を返す。
// 登録の途中の設定も返すため、有効かどうかは呼び出し側が IsEnabled で確かめる。
func (r *UserTwoFactorAuthRepository) FindByUserID(ctx context.Context, userID model.UserID) (*model.UserTwoFactorAuth, error) {
	row, err := r.q.GetUserTwoFactorAuthByUserID(ctx, uuid.UUID(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// Enable は登録の途中の設定を有効にし、照合したコードのタイムステップを使用済みとして記録する。
// 登録の途中の設定が無い・既に有効・照合後に秘密鍵が差し替わったときは更新せずfalseを返す。
func (r *UserTwoFactorAuthRepository) Enable(ctx context.Context, userID model.UserID, expectedSecretCiphertext []byte, step int64) (bool, error) {
	affected, err := r.q.EnableUserTwoFactorAuth(ctx, query.EnableUserTwoFactorAuthParams{
		UserID:                   uuid.UUID(userID),
		ExpectedSecretCiphertext: expectedSecretCiphertext,
		Step:                     step,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// UseStep は有効な設定に、受け付けたコードのタイムステップを使用済みとして記録する。
// 記録済みのステップ以前のコードのときは更新せずfalseを返し、呼び出し側はコードを拒否する。
//
// 照合と記録を分けずに条件付きのUPDATEで判定するのは、同じコードを同時に送られても一方しか通さないため。
func (r *UserTwoFactorAuthRepository) UseStep(ctx context.Context, userID model.UserID, step int64) (bool, error) {
	affected, err := r.q.UpdateUserTwoFactorAuthLastUsedStep(ctx, query.UpdateUserTwoFactorAuthLastUsedStepParams{
		UserID: uuid.UUID(userID),
		Step:   step,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// UseMatchingStep は照合した有効な設定に限ってタイムステップを記録する。
// 設定が入れ替わったときやコードが使用済みのときはfalseを返す。
func (r *UserTwoFactorAuthRepository) UseMatchingStep(ctx context.Context, setting *model.UserTwoFactorAuth, step int64) (bool, error) {
	affected, err := r.q.UseMatchingUserTwoFactorAuthStep(ctx, query.UseMatchingUserTwoFactorAuthStepParams{
		UserID:                   uuid.UUID(setting.UserID),
		ExpectedID:               uuid.UUID(setting.ID),
		ExpectedSecretCiphertext: setting.SecretCiphertext,
		Step:                     step,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteMatching は再認証した有効な設定だけを削除する。
// 照合後に設定が入れ替わったときはfalseを返す。
func (r *UserTwoFactorAuthRepository) DeleteMatching(ctx context.Context, setting *model.UserTwoFactorAuth) (bool, error) {
	affected, err := r.q.DeleteMatchingUserTwoFactorAuth(ctx, query.DeleteMatchingUserTwoFactorAuthParams{
		UserID:                   uuid.UUID(setting.UserID),
		ExpectedID:               uuid.UUID(setting.ID),
		ExpectedSecretCiphertext: setting.SecretCiphertext,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteByUserID はユーザーの二要素認証の設定を削除する。
func (r *UserTwoFactorAuthRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeleteUserTwoFactorAuthByUserID(ctx, uuid.UUID(userID))
}

// toModel はクエリの行を model.UserTwoFactorAuth に変換する。
func (r *UserTwoFactorAuthRepository) toModel(row query.UserTwoFactorAuth) *model.UserTwoFactorAuth {
	return &model.UserTwoFactorAuth{
		ID:               model.UserTwoFactorAuthID(row.ID),
		UserID:           model.UserID(row.UserID),
		SecretCiphertext: row.SecretCiphertext,
		LastUsedStep:     row.LastUsedStep,
		EnabledAt:        row.EnabledAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}
