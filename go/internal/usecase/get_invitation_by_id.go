package usecase

import (
	"context"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetInvitationByIDUsecase は、登録の手順の間Cookieが運ぶ招待のIDから、登録に使える招待を引く。
// 招待リンクを開いてから時間が経つため、登録の各手順で改めて使えることを確かめるのに使う。
type GetInvitationByIDUsecase struct {
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
}

// NewGetInvitationByIDUsecase は GetInvitationByIDUsecase を生成する。
func NewGetInvitationByIDUsecase(
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
) *GetInvitationByIDUsecase {
	return &GetInvitationByIDUsecase{invitationRepo: invitationRepo, invitationRedemptionRepo: invitationRedemptionRepo}
}

// GetInvitationByIDOutput は GetInvitationByIDUsecase.Execute の結果。
type GetInvitationByIDOutput struct {
	Invitation *model.Invitation
}

// Execute はIDの招待を引き、登録に使えるときだけ返す。
// 使えないときの扱いは GetInvitationUsecase と同じ (AppErrCodeResourceNotFound)。
func (uc *GetInvitationByIDUsecase) Execute(ctx context.Context, id model.InvitationID) (*GetInvitationByIDOutput, error) {
	invitation, err := uc.invitationRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	if err := checkUsableInvitation(ctx, uc.invitationRedemptionRepo, invitation); err != nil {
		return nil, err
	}

	return &GetInvitationByIDOutput{Invitation: invitation}, nil
}
