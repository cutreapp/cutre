package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// GetInvitationUsecase は招待リンクのトークンから、登録に使える招待と招待した人を引く。
type GetInvitationUsecase struct {
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
	userRepo                 *repository.UserRepository
}

// NewGetInvitationUsecase は GetInvitationUsecase を生成する。
func NewGetInvitationUsecase(
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
	userRepo *repository.UserRepository,
) *GetInvitationUsecase {
	return &GetInvitationUsecase{invitationRepo: invitationRepo, invitationRedemptionRepo: invitationRedemptionRepo, userRepo: userRepo}
}

// GetInvitationInput は GetInvitationUsecase.Execute の入力。
type GetInvitationInput struct {
	// Token は招待リンクに載っていたトークン。
	Token string
}

// GetInvitationOutput は GetInvitationUsecase.Execute の結果。
type GetInvitationOutput struct {
	Invitation *model.Invitation
	// Inviter は招待したユーザー。管理者が発行した招待と、招待した人が退会した招待ではnilになる。
	// どちらかは Invitation.InviterUserID の有無で見分ける。
	Inviter *model.User
}

// Execute はトークンの招待を引き、登録に使えるときだけ返す。
//
// 無い招待と、期限切れ・取り消し済み・人数の上限に達した招待は区別せず AppErrCodeResourceNotFound を返す。
// 利用者に求めることはどれも「招待した人に確かめる」で同じになるため。
func (uc *GetInvitationUsecase) Execute(ctx context.Context, input GetInvitationInput) (*GetInvitationOutput, error) {
	invitation, err := uc.invitationRepo.FindByToken(ctx, input.Token)
	if err != nil {
		return nil, fmt.Errorf("招待の取得に失敗: %w", err)
	}
	if err := checkUsableInvitation(ctx, uc.invitationRedemptionRepo, invitation); err != nil {
		return nil, err
	}

	output := &GetInvitationOutput{Invitation: invitation}
	if invitation.InviterUserID != nil {
		inviter, err := uc.userRepo.FindByID(ctx, *invitation.InviterUserID)
		if err != nil {
			return nil, fmt.Errorf("招待した人の取得に失敗: %w", err)
		}
		output.Inviter = inviter
	}

	return output, nil
}

// checkUsableInvitation は、招待が無いか登録に使えないときに AppErrCodeResourceNotFound を返す。
// 招待リンクから引くときも、Cookieに入れたIDから引くときも、同じ基準で判定する。
func checkUsableInvitation(ctx context.Context, invitationRedemptionRepo *repository.InvitationRedemptionRepository, invitation *model.Invitation) error {
	usable, err := isUsableInvitation(ctx, invitationRedemptionRepo, invitation)
	if err != nil {
		return err
	}
	if !usable {
		return &model.AppError{
			Code:    model.AppErrCodeResourceNotFound,
			UserMsg: i18n.T(ctx, "invitation_acceptance_new_unusable_message"),
		}
	}

	return nil
}

// isUsableInvitation は、招待があり、今の時点で登録に使えるかを返す。
// 人数の空きは、招待の種類ごとに Invitation.RedemptionLimit と同じ数え方で数える。
func isUsableInvitation(ctx context.Context, invitationRedemptionRepo *repository.InvitationRedemptionRepository, invitation *model.Invitation) (bool, error) {
	if invitation == nil {
		return false, nil
	}

	var count int
	var err error
	if invitation.IsAdminIssued() {
		count, err = invitationRedemptionRepo.CountByInvitationID(ctx, invitation.ID)
	} else {
		count, err = invitationRedemptionRepo.CountByInviterUserID(ctx, *invitation.InviterUserID)
	}
	if err != nil {
		return false, fmt.Errorf("招待の使用の件数の取得に失敗: %w", err)
	}

	return invitation.IsUsable(time.Now(), count), nil
}
