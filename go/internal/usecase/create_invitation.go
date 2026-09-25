package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// CreateInvitationUsecase は招待者の無い招待を発行する。
//
// 最初のユーザーの登録と、運営からの招待に使う。管理者がCLIから実行するため、
// 招待枠の判定も利用者の入力の検証も持たない。
type CreateInvitationUsecase struct {
	invitationRepo *repository.InvitationRepository
}

// NewCreateInvitationUsecase は CreateInvitationUsecase を生成する。
func NewCreateInvitationUsecase(invitationRepo *repository.InvitationRepository) *CreateInvitationUsecase {
	return &CreateInvitationUsecase{invitationRepo: invitationRepo}
}

// CreateInvitationOutput は CreateInvitationUsecase.Execute の結果。
type CreateInvitationOutput struct {
	Invitation *model.Invitation
}

// Execute は招待リンクのトークンを発行し、有効期限付きで招待を保存する。
// 書き込みは1回のためトランザクションは開かない。
func (uc *CreateInvitationUsecase) Execute(ctx context.Context) (*CreateInvitationOutput, error) {
	token, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("招待のトークンの生成に失敗: %w", err)
	}

	invitation, err := uc.invitationRepo.Create(ctx, repository.CreateInvitationInput{
		Token:     token,
		ExpiresAt: model.InvitationExpiresAt(time.Now()),
	})
	if err != nil {
		return nil, fmt.Errorf("招待の保存に失敗: %w", err)
	}

	return &CreateInvitationOutput{Invitation: invitation}, nil
}
