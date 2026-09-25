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

// UserSessionRepository はuser_sessionsを読み書きする。
type UserSessionRepository struct {
	q *query.Queries
}

// NewUserSessionRepository は UserSessionRepository を生成する。
func NewUserSessionRepository(db *sql.DB) *UserSessionRepository {
	return &UserSessionRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい UserSessionRepository を返す。
// ログインとログアウトのように、セッションと他の行を1つのトランザクションで扱うUseCaseがこれを使う。
func (r *UserSessionRepository) WithTx(tx *sql.Tx) *UserSessionRepository {
	return &UserSessionRepository{q: r.q.WithTx(tx)}
}

// FindLiveWithUserByTokenDigest は指定したダイジェストのセッションを、持ち主のユーザーを添えて返す。
// 該当が無い場合は (nil, nil) を返す。
//
// 期限切れのセッションと、退会したユーザーのセッションは該当しないものとして扱う。
// 未存在は正常なルックアップの結果 (失効した / 偽造されたCookie) であり、呼び出し側は未ログインとして扱う。
func (r *UserSessionRepository) FindLiveWithUserByTokenDigest(ctx context.Context, tokenDigest string) (*model.UserSession, error) {
	row, err := r.q.GetLiveUserSessionWithUserByTokenDigest(ctx, tokenDigest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	session := r.toModel(row.UserSession)
	session.User = toUserModel(row.User)

	return session, nil
}

// CreateUserSessionInput はセッションの作成に必要な属性。
// idとタイムスタンプ (last_seen_at・signed_in_at・created_at・updated_at) はデータベースが採番する。
//
// TokenDigest は既にダイジェストである必要があり、このリポジトリは平文のトークンを扱わない。
type CreateUserSessionInput struct {
	UserID      model.UserID
	TokenDigest string
	ExpiresAt   time.Time
	IPAddress   string
	UserAgent   string
}

// Create はセッションを挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *UserSessionRepository) Create(ctx context.Context, input CreateUserSessionInput) (*model.UserSession, error) {
	row, err := r.q.CreateUserSession(ctx, query.CreateUserSessionParams{
		UserID:      uuid.UUID(input.UserID),
		TokenDigest: input.TokenDigest,
		ExpiresAt:   input.ExpiresAt,
		IpAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// Extend はセッションの有効期限と最終利用時刻を進める。
// 既に消えているセッションを延長してもエラーにならないため、呼び出し側は事前の存在確認をしなくてよい。
func (r *UserSessionRepository) Extend(ctx context.Context, id model.UserSessionID, expiresAt, lastSeenAt time.Time) error {
	return r.q.ExtendUserSession(ctx, query.ExtendUserSessionParams{
		ID:         uuid.UUID(id),
		ExpiresAt:  expiresAt,
		LastSeenAt: lastSeenAt,
	})
}

// DeleteByTokenDigest は指定したダイジェストのセッションを削除する。
// 既に消えているセッションの削除はエラーにならないため、二重のログアウトも無害である。
func (r *UserSessionRepository) DeleteByTokenDigest(ctx context.Context, tokenDigest string) error {
	return r.q.DeleteUserSessionByTokenDigest(ctx, tokenDigest)
}

// DeleteByUserID は指定したユーザーのセッションをすべて削除し、全端末からログアウトさせる。
// 退会とパスワードの変更で使う。
func (r *UserSessionRepository) DeleteByUserID(ctx context.Context, userID model.UserID) error {
	return r.q.DeleteUserSessionsByUserID(ctx, uuid.UUID(userID))
}

// DeleteExpired はnowまでに有効期限が切れたセッションを削除する。
// 期限切れのセッションはログインに使われないため、残しておく必要がない。
func (r *UserSessionRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	return r.q.DeleteExpiredUserSessions(ctx, now)
}

// toModel はクエリの行を model.UserSession に変換する。
func (r *UserSessionRepository) toModel(row query.UserSession) *model.UserSession {
	return &model.UserSession{
		ID:          model.UserSessionID(row.ID),
		UserID:      model.UserID(row.UserID),
		TokenDigest: row.TokenDigest,
		ExpiresAt:   row.ExpiresAt,
		LastSeenAt:  row.LastSeenAt,
		IPAddress:   row.IpAddress,
		UserAgent:   row.UserAgent,
		SignedInAt:  row.SignedInAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
