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

// InvitationRepository はinvitationsを読み書きする。
type InvitationRepository struct {
	q *query.Queries
}

// NewInvitationRepository は InvitationRepository を生成する。
func NewInvitationRepository(db *sql.DB) *InvitationRepository {
	return &InvitationRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい InvitationRepository を返す。
// 招待をロックして読んでから、登録の開始やアカウントの作成を行うUseCaseがこれを使う。
func (r *InvitationRepository) WithTx(tx *sql.Tx) *InvitationRepository {
	return &InvitationRepository{q: r.q.WithTx(tx)}
}

// CreateInvitationInput は招待の作成に必要な属性。
// idとタイムスタンプ (created_at・updated_at) はデータベースが採番する。
type CreateInvitationInput struct {
	// InviterUserID は招待したユーザー。nilは管理者がCLIから発行した招待を表す。
	InviterUserID *model.UserID
	Token         string
	ExpiresAt     time.Time
}

// Create は招待を挿入し、データベースが採番したidとタイムスタンプを含めて返す。
func (r *InvitationRepository) Create(ctx context.Context, input CreateInvitationInput) (*model.Invitation, error) {
	row, err := r.q.CreateInvitation(ctx, query.CreateInvitationParams{
		InviterUserID: userIDToNullableUUID(input.InviterUserID),
		Token:         input.Token,
		ExpiresAt:     input.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}

	return r.toModel(row), nil
}

// FindByIDForUpdate は招待の行を排他ロック付きで返す。存在しない場合は (nil, nil) を返す。
// 同じ招待でのアカウントの作成どうしと、取り消しを、トランザクションが終わるまで待たせる。
func (r *InvitationRepository) FindByIDForUpdate(ctx context.Context, id model.InvitationID) (*model.Invitation, error) {
	row, err := r.q.GetInvitationByIDForUpdate(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// FindByID は指定したIDの招待を返す。存在しない場合は (nil, nil) を返す。
// FindByToken と同じく、使えるかどうかの判定は呼び出し側に任せる。
func (r *InvitationRepository) FindByID(ctx context.Context, id model.InvitationID) (*model.Invitation, error) {
	row, err := r.q.GetInvitationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// FindByIDForShare は招待の行を更新と競合する共有ロック付きで返す。
// 登録開始のトランザクション中に、取り消しと同じ招待でのアカウントの作成 (FindByIDForUpdate) が割り込まないようにする。
func (r *InvitationRepository) FindByIDForShare(ctx context.Context, id model.InvitationID) (*model.Invitation, error) {
	row, err := r.q.GetInvitationByIDForShare(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return r.toModel(row), nil
}

// FindByToken は指定したトークンの招待を返す。存在しない場合は (nil, nil) を返す。
// 期限切れ・取り消し済みの招待も返し、使えるかどうかの判定は呼び出し側に任せる。
func (r *InvitationRepository) FindByToken(ctx context.Context, token string) (*model.Invitation, error) {
	row, err := r.q.GetInvitationByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// FindUnrevokedByInviterUserID は招待者の取り消していない招待を返す。無い場合は (nil, nil) を返す。
// 取り消していない招待は招待者ごとに1本に限られる。期限の切れた招待も返し、使えるかどうかの判定は呼び出し側に任せる。
func (r *InvitationRepository) FindUnrevokedByInviterUserID(ctx context.Context, inviterUserID model.UserID) (*model.Invitation, error) {
	id := uuid.UUID(inviterUserID)
	row, err := r.q.GetUnrevokedInvitationByInviterUserID(ctx, &id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// CreateUnlessUnrevokedExists は、招待者が取り消していない招待を持たないときだけ招待を挿入して返す。
// 既に持つときは挿入せず (nil, nil) を返す。同じ招待者の招待を同時に作ろうとしたときに、エラーにせず先に作られた方へ揃えるため。
func (r *InvitationRepository) CreateUnlessUnrevokedExists(ctx context.Context, input CreateInvitationInput) (*model.Invitation, error) {
	row, err := r.q.CreateInvitationUnlessUnrevokedExists(ctx, query.CreateInvitationUnlessUnrevokedExistsParams{
		InviterUserID: userIDToNullableUUID(input.InviterUserID),
		Token:         input.Token,
		ExpiresAt:     input.ExpiresAt,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return r.toModel(row), nil
}

// Revoke は招待を取り消す。取り消し済みの招待は取り消した時刻を変えない。
func (r *InvitationRepository) Revoke(ctx context.Context, id model.InvitationID) error {
	return r.q.RevokeInvitation(ctx, uuid.UUID(id))
}

// RevokeUnrevokedByInviterUserID は招待者の取り消していない招待をすべて取り消す。取り消し済みの招待は取り消した時刻を変えない。
func (r *InvitationRepository) RevokeUnrevokedByInviterUserID(ctx context.Context, inviterUserID model.UserID) error {
	id := uuid.UUID(inviterUserID)
	return r.q.RevokeUnrevokedInvitationsByInviterUserID(ctx, &id)
}

// toModel はクエリの行を model.Invitation に変換する。
func (r *InvitationRepository) toModel(row query.Invitation) *model.Invitation {
	return &model.Invitation{
		ID:            model.InvitationID(row.ID),
		InviterUserID: toNullableUserID(row.InviterUserID),
		Token:         row.Token,
		ExpiresAt:     row.ExpiresAt,
		RevokedAt:     row.RevokedAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// userIDToNullableUUID はNULL許容のユーザーIDを、クエリの引数の *uuid.UUID に変換する。
func userIDToNullableUUID(id *model.UserID) *uuid.UUID {
	if id == nil {
		return nil
	}

	u := uuid.UUID(*id)
	return &u
}

// toNullableUserID はNULL許容の列の *uuid.UUID を *model.UserID に変換する。
func toNullableUserID(id *uuid.UUID) *model.UserID {
	if id == nil {
		return nil
	}

	userID := model.UserID(*id)
	return &userID
}
