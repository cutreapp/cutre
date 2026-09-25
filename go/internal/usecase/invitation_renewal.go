package usecase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// renewInvitation は、招待者の今の招待 (current) を取り消して新しい招待を作り、招待者の使える招待を返す。
// current がnilなら取り消さずに作る。人数の上限に達しているか、招待者が退会していれば作らずにnilを返す。
// created は、この呼び出しで新しい招待を作ったかを示す。先に作られた招待に揃えたときはfalse。
//
// 取り消していない招待を招待者ごとに1本に限る制約があるため、取り消しと作成を1つのトランザクションで行う。
// 同じ招待者の別のリクエストが先に作り直していれば、後から作ろうとした側はその招待を返し、使える招待は1本に収束する。
func renewInvitation(
	ctx context.Context,
	db *sql.DB,
	userRepo *repository.UserRepository,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
	inviter *model.User,
	current *model.Invitation,
) (*model.Invitation, bool, error) {
	loc, err := inviter.Location()
	if err != nil {
		return nil, false, fmt.Errorf("招待者のタイムゾーンの読み込みに失敗: %w", err)
	}
	token, err := auth.GenerateSecureToken()
	if err != nil {
		return nil, false, fmt.Errorf("招待のトークンの生成に失敗: %w", err)
	}

	inviterID := inviter.ID
	invitation, latestCount, err := replaceInvitation(ctx, db, userRepo, invitationRepo, invitationRedemptionRepo, current, repository.CreateInvitationInput{
		InviterUserID: &inviterID,
		Token:         token,
		ExpiresAt:     model.UserInvitationExpiresAt(time.Now(), loc),
	})
	if errors.Is(err, errInviterWithdrawn) {
		// 画面を開いた後に招待者が退会した。退会したユーザーの招待は使えないため、招待は無いものとして返す。
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if model.InviterRemainingRedemptions(latestCount) == 0 {
		return nil, false, nil
	}
	created := invitation != nil
	if !created {
		// 同じ招待者の別のリクエストが先に招待を作った。その招待に揃える。
		invitation, err = invitationRepo.FindUnrevokedByInviterUserID(ctx, inviterID)
		if err != nil {
			return nil, false, fmt.Errorf("先に作られた招待の取得に失敗: %w", err)
		}
		if invitation == nil {
			return nil, false, fmt.Errorf("招待の作成が競合したが、取り消していない招待が見つからない: inviter_user_id=%s", inviterID)
		}
	}

	if !invitation.IsUsable(time.Now(), latestCount) {
		return nil, false, nil
	}
	return invitation, created, nil
}

// errInviterWithdrawn は、招待を作ろうとした招待者が退会していたことを表す。
var errInviterWithdrawn = errors.New("招待者が退会している")

// replaceInvitation は、今の招待 (current) があれば取り消し、新しい招待を作る。
// 招待者が取り消していない招待を既に持つか、人数の上限に達したときは作らずにnilを返す。
// 招待者が退会していたときは errInviterWithdrawn を返す。
// 登録の処理と同じ招待行をロックしてから人数を数え直し、上限到達後の作成を防ぐ。
//
// 招待行より先に招待者の行をロックする。退会 (DeleteAccountUsecase) もユーザーの行を先に更新してから招待を取り消すため、
// ロックの順序が揃ってデッドロックせず、退会の取り消しの後に招待が作られて残ることもない。
func replaceInvitation(
	ctx context.Context,
	db *sql.DB,
	userRepo *repository.UserRepository,
	invitationRepo *repository.InvitationRepository,
	invitationRedemptionRepo *repository.InvitationRedemptionRepository,
	current *model.Invitation,
	input repository.CreateInvitationInput,
) (*model.Invitation, int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	lockedInviter, err := userRepo.WithTx(tx).LockByID(ctx, *input.InviterUserID)
	if err != nil {
		return nil, 0, fmt.Errorf("招待者のロックに失敗: %w", err)
	}
	if lockedInviter == nil {
		return nil, 0, errInviterWithdrawn
	}

	txInvitationRepo := invitationRepo.WithTx(tx)

	if current != nil {
		current, err = txInvitationRepo.FindByIDForUpdate(ctx, current.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("招待のロックに失敗: %w", err)
		}
	}
	redeemedCount, err := invitationRedemptionRepo.WithTx(tx).CountByInviterUserID(ctx, *input.InviterUserID)
	if err != nil {
		return nil, 0, fmt.Errorf("招待の使用の件数の再取得に失敗: %w", err)
	}
	if model.InviterRemainingRedemptions(redeemedCount) == 0 {
		return nil, redeemedCount, nil
	}

	if current != nil && current.RevokedAt == nil {
		if err := txInvitationRepo.Revoke(ctx, current.ID); err != nil {
			return nil, 0, fmt.Errorf("今の招待の取り消しに失敗: %w", err)
		}
	}
	invitation, err := txInvitationRepo.CreateUnlessUnrevokedExists(ctx, input)
	if err != nil {
		return nil, 0, fmt.Errorf("招待の保存に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return invitation, redeemedCount, nil
}
