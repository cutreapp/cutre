package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// PrepareInvitationUsecase は招待の画面に出す招待を用意する。
//
// ユーザーは使える招待を常に1本持つ。画面を開いたときに使える招待が無く、人数に空きがあれば、その場で新しい招待を作る。
// 人数に空きが無ければ招待を作らない (どの招待も使えないため)。
type PrepareInvitationUsecase struct {
	db                       *sql.DB
	userRepo                 *repository.UserRepository
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
}

// NewPrepareInvitationUsecase は PrepareInvitationUsecase を生成する。
func NewPrepareInvitationUsecase(
	db *sql.DB,
	userRepo *repository.UserRepository,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
) *PrepareInvitationUsecase {
	return &PrepareInvitationUsecase{db: db, userRepo: userRepo, invitationRepo: invitationRepo, invitationRedemptionRepo: invitationRedemptionRepo}
}

// PrepareInvitationInput は PrepareInvitationUsecase.Execute の入力。
type PrepareInvitationInput struct {
	// Inviter は招待するユーザー。招待の期限をこのユーザーのタイムゾーンの日付で決める。
	Inviter *model.User
}

// PrepareInvitationOutput は PrepareInvitationUsecase.Execute の結果。
type PrepareInvitationOutput struct {
	// Invitation は使える招待。人数の上限に達しているときはnil。
	Invitation *model.Invitation
}

// Execute は招待者の使える招待を返し、無ければ作る。
//
// 期限の切れた招待は取り消してから作り直す。2つのタブから同時に開いても、使える招待は1本に収束する。
func (uc *PrepareInvitationUsecase) Execute(ctx context.Context, input PrepareInvitationInput) (*PrepareInvitationOutput, error) {
	inviterID := input.Inviter.ID

	redeemedCount, err := uc.invitationRedemptionRepo.CountByInviterUserID(ctx, inviterID)
	if err != nil {
		return nil, fmt.Errorf("招待の使用の件数の取得に失敗: %w", err)
	}
	if model.InviterRemainingRedemptions(redeemedCount) == 0 {
		return &PrepareInvitationOutput{}, nil
	}

	current, err := uc.invitationRepo.FindUnrevokedByInviterUserID(ctx, inviterID)
	if err != nil {
		return nil, fmt.Errorf("取り消していない招待の取得に失敗: %w", err)
	}
	if current != nil && current.IsUsable(time.Now(), redeemedCount) {
		return &PrepareInvitationOutput{Invitation: current}, nil
	}

	invitation, _, err := renewInvitation(ctx, uc.db, uc.userRepo, uc.invitationRepo, uc.invitationRedemptionRepo, input.Inviter, current)
	if err != nil {
		return nil, err
	}
	return &PrepareInvitationOutput{Invitation: invitation}, nil
}
