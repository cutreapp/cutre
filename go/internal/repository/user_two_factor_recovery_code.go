package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// UserTwoFactorRecoveryCodeRepository はuser_two_factor_recovery_codesを読み書きする。
//
// コードはダイジェストのまま受け渡し、このリポジトリは平文のコードを扱わない。
type UserTwoFactorRecoveryCodeRepository struct {
	q *query.Queries
}

// NewUserTwoFactorRecoveryCodeRepository は UserTwoFactorRecoveryCodeRepository を生成する。
func NewUserTwoFactorRecoveryCodeRepository(db *sql.DB) *UserTwoFactorRecoveryCodeRepository {
	return &UserTwoFactorRecoveryCodeRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserTwoFactorRecoveryCodeRepository を返す。
// コードの差し替えや、コードの使用とセッションの発行を1つのトランザクションで行うUseCaseがこれを使う。
func (r *UserTwoFactorRecoveryCodeRepository) WithTx(tx *sql.Tx) *UserTwoFactorRecoveryCodeRepository {
	return &UserTwoFactorRecoveryCodeRepository{q: r.q.WithTx(tx)}
}

// CreateAll はユーザーのリカバリーコードをダイジェストの数だけ作る。
// 途中で失敗すると一部だけが残るため、呼び出し側はトランザクションの中で呼ぶ。
func (r *UserTwoFactorRecoveryCodeRepository) CreateAll(ctx context.Context, userID model.UserID, codeDigests []string) error {
	for _, digest := range codeDigests {
		if err := r.q.CreateUserTwoFactorRecoveryCode(ctx, query.CreateUserTwoFactorRecoveryCodeParams{
			UserID:     uuid.UUID(userID),
			CodeDigest: digest,
		}); err != nil {
			return err
		}
	}

	return nil
}

// CountUnused はユーザーのまだ使っていないリカバリーコードの数を返す。
func (r *UserTwoFactorRecoveryCodeRepository) CountUnused(ctx context.Context, userID model.UserID) (int, error) {
	count, err := r.q.CountUnusedUserTwoFactorRecoveryCodes(ctx, uuid.UUID(userID))
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// Use はダイジェストが一致する未使用のコードを使用済みにする。
// 一致するコードが無い・使用済みのときは更新せずfalseを返す。
//
// 照合と使用を分けずに条件付きのUPDATEで判定するのは、同じコードを同時に送られても一方しか通さないため。
func (r *UserTwoFactorRecoveryCodeRepository) Use(ctx context.Context, userID model.UserID, codeDigest string) (bool, error) {
	affected, err := r.q.UseUserTwoFactorRecoveryCode(ctx, query.UseUserTwoFactorRecoveryCodeParams{
		UserID:     uuid.UUID(userID),
		CodeDigest: codeDigest,
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteByUserID はユーザーのリカバリーコードをすべて削除する。
func (r *UserTwoFactorRecoveryCodeRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeleteUserTwoFactorRecoveryCodesByUserID(ctx, uuid.UUID(userID))
}
