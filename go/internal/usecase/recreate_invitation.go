package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// RecreateInvitationUsecase は招待リンクを作り直す。
//
// 今の招待を取り消して新しい招待を作る。渡したくない相手にリンクが届いたときに、そのリンクで登録させないため。
// 取り消した招待で登録の途中にいる人は、最後のアカウント作成で失敗する。
type RecreateInvitationUsecase struct {
	db                       *sql.DB
	userRepo                 *repository.UserRepository
	invitationRepo           *repository.InvitationRepository
	invitationRedemptionRepo *repository.InvitationRedemptionRepository
}

// NewRecreateInvitationUsecase は RecreateInvitationUsecase を生成する。
func NewRecreateInvitationUsecase(
	db *sql.DB,
	userRepo *repository.UserRepository,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
) *RecreateInvitationUsecase {
	return &RecreateInvitationUsecase{db: db, userRepo: userRepo, invitationRepo: invitationRepo, invitationRedemptionRepo: invitationRedemptionRepo}
}

// RecreateInvitationInput は RecreateInvitationUsecase.Execute の入力。
type RecreateInvitationInput struct {
	// Inviter は招待するユーザー。新しい招待の期限をこのユーザーのタイムゾーンの日付で決める。
	Inviter *model.User
	// CurrentInvitationID は画面に表示した招待のID。作り直す対象で、二重送信を見分けるのにも使う。
	CurrentInvitationID model.InvitationID
}

// RecreateInvitationOutput は RecreateInvitationUsecase.Execute の結果。
type RecreateInvitationOutput struct {
	// Invitation は作り直した招待。二重送信では先に作られた招待。人数の上限に達しているときはnil。
	Invitation *model.Invitation
	// Recreated は、この呼び出しで作り直したか。表示した招待が既に作り直されていて、先に作られた招待に揃えたときはfalse。
	Recreated bool
}

// Execute は画面に表示した招待を取り消し、新しい招待を作る。
//
// 人数の上限に達していれば、今の招待を取り消さずにそのまま残す。上限に達した招待はもう使えないため。
// 同じ招待IDで二重に送信されても、先に作られた招待を返し、作り直しは1回に収まる。
func (uc *RecreateInvitationUsecase) Execute(ctx context.Context, input RecreateInvitationInput) (*RecreateInvitationOutput, error) {
	current, err := uc.invitationRepo.FindByID(ctx, input.CurrentInvitationID)
	if err != nil {
		return nil, fmt.Errorf("表示した招待の取得に失敗: %w", err)
	}
	if current == nil || current.InviterUserID == nil || *current.InviterUserID != input.Inviter.ID {
		return nil, &model.AppError{Code: model.AppErrCodeForbidden}
	}

	invitation, recreated, err := renewInvitation(ctx, uc.db, uc.userRepo, uc.invitationRepo, uc.invitationRedemptionRepo, input.Inviter, current)
	if err != nil {
		return nil, err
	}
	return &RecreateInvitationOutput{Invitation: invitation, Recreated: recreated}, nil
}
