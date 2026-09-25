package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetInvitationRedemptionsUsecase は、ユーザーの招待で登録した人 (参加した人) の一覧を引く。
type GetInvitationRedemptionsUsecase struct {
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
}

// NewGetInvitationRedemptionsUsecase は GetInvitationRedemptionsUsecase を生成する。
func NewGetInvitationRedemptionsUsecase(invitationRedemptionRepo *repository.InvitationRedemptionRepository) *GetInvitationRedemptionsUsecase {
	return &GetInvitationRedemptionsUsecase{invitationRedemptionRepo: invitationRedemptionRepo}
}

// GetInvitationRedemptionsInput は GetInvitationRedemptionsUsecase.Execute の入力。
type GetInvitationRedemptionsInput struct {
	InviterUserID model.UserID
}

// GetInvitationRedemptionsOutput は GetInvitationRedemptionsUsecase.Execute の結果。
type GetInvitationRedemptionsOutput struct {
	// Redemptions は招待者のすべての招待の使用で、新しい順に並ぶ。
	// 各要素の User に登録した人を持ち、退会した人は User.DeletedAt が入る。
	Redemptions []*model.InvitationRedemption
}

// Execute は招待者の招待で登録した人を、退会した人も含めて新しい順に返す。
func (uc *GetInvitationRedemptionsUsecase) Execute(ctx context.Context, input GetInvitationRedemptionsInput) (*GetInvitationRedemptionsOutput, error) {
	redemptions, err := uc.invitationRedemptionRepo.ListWithUsersByInviterUserID(ctx, input.InviterUserID)
	if err != nil {
		return nil, fmt.Errorf("招待の使用の一覧の取得に失敗: %w", err)
	}

	return &GetInvitationRedemptionsOutput{Redemptions: redemptions}, nil
}
