package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/query"
)

// InvitationRedemptionRepository はinvitation_redemptionsを読み書きする。
type InvitationRedemptionRepository struct {
	q *query.Queries
}

// NewInvitationRedemptionRepository は InvitationRedemptionRepository を生成する。
func NewInvitationRedemptionRepository(db *sql.DB) *InvitationRedemptionRepository {
	return &InvitationRedemptionRepository{q: query.New(db)}
}

// WithTx はクエリをtx内で実行する新しい InvitationRedemptionRepository を返す。
// アカウントの作成と招待の使用の記録を1つのトランザクションで行うUseCaseがこれを使う。
func (r *InvitationRedemptionRepository) WithTx(tx *sql.Tx) *InvitationRedemptionRepository {
	return &InvitationRedemptionRepository{q: r.q.WithTx(tx)}
}

// Create は invitationID の招待を userID のユーザーが使った記録を挿入する。
func (r *InvitationRedemptionRepository) Create(ctx context.Context, invitationID model.InvitationID, userID model.UserID) (*model.InvitationRedemption, error) {
	row, err := r.q.CreateInvitationRedemption(ctx, query.CreateInvitationRedemptionParams{
		InvitationID: uuid.UUID(invitationID),
		UserID:       uuid.UUID(userID),
	})
	if err != nil {
		return nil, err
	}

	return &model.InvitationRedemption{
		ID:           model.InvitationRedemptionID(row.ID),
		InvitationID: model.InvitationID(row.InvitationID),
		UserID:       model.UserID(row.UserID),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

// CountByInvitationID は招待1つの使用の件数を返す。
func (r *InvitationRedemptionRepository) CountByInvitationID(ctx context.Context, invitationID model.InvitationID) (int, error) {
	count, err := r.q.CountInvitationRedemptionsByInvitationID(ctx, uuid.UUID(invitationID))
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// CountByInviterUserID は招待者のすべての招待 (取り消した・期限の切れた招待を含む) の使用の件数を返す。
func (r *InvitationRedemptionRepository) CountByInviterUserID(ctx context.Context, inviterUserID model.UserID) (int, error) {
	id := uuid.UUID(inviterUserID)
	count, err := r.q.CountInvitationRedemptionsByInviterUserID(ctx, &id)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// ListWithUsersByInviterUserID は、招待者のすべての招待 (取り消した・期限の切れた招待を含む) の使用を、
// 登録したユーザーと合わせて新しい順に返す。
//
// 退会したユーザーも匿名化した行が残るため含め、User.DeletedAt で見分けられるようにする。
// User にはアットネームと退会した時刻だけを持たせる。
func (r *InvitationRedemptionRepository) ListWithUsersByInviterUserID(ctx context.Context, inviterUserID model.UserID) ([]*model.InvitationRedemption, error) {
	id := uuid.UUID(inviterUserID)
	rows, err := r.q.ListInvitationRedemptionsWithUsersByInviterUserID(ctx, &id)
	if err != nil {
		return nil, err
	}

	redemptions := make([]*model.InvitationRedemption, len(rows))
	for i, row := range rows {
		redemptions[i] = &model.InvitationRedemption{
			ID:           model.InvitationRedemptionID(row.ID),
			InvitationID: model.InvitationID(row.InvitationID),
			UserID:       model.UserID(row.UserID),
			User: &model.User{
				ID:        model.UserID(row.UserID),
				Atname:    row.UserAtname,
				DeletedAt: row.UserDeletedAt,
			},
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}

	return redemptions, nil
}
