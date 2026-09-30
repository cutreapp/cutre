package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// DeleteAccountUsecase は、今のパスワードで本人であることを確かめてから退会させる。
//
// usersの行は消さずに残し (tombstone)、メールアドレスとアットネームを匿名の値に置き換える。
// 招待の経路 (誰が誰を招待したか) と、交換の記録・メッセージを退会の後も相手が辿れるようにするため。
// 認証の情報とメールアドレスを含む行は削除し、個人を特定できる情報を残さない。
// リストのアイテムは交換の品から辿れるよう行を残して外し、交換場所は消す。
// 進行中の交換があるときは退会させない。
type DeleteAccountUsecase struct {
	db                            *sql.DB
	withdrawalDeleteValidator     *validator.WithdrawalDeleteValidator
	userRepo                      *repository.UserRepository
	userPasswordRepo              *repository.UserPasswordRepository
	userSessionRepo               *repository.UserSessionRepository
	userTwoFactorAuthRepo         *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository
	passwordResetTokenRepo        *repository.PasswordResetTokenRepository
	emailConfirmationRepo         *repository.EmailConfirmationRepository
	invitationRepo                *repository.InvitationRepository
	itemRepo                      *repository.ItemRepository
	userStationRepo               *repository.UserStationRepository
}

// NewDeleteAccountUsecase は DeleteAccountUsecase を生成する。
func NewDeleteAccountUsecase(
	db *sql.DB,
	withdrawalDeleteValidator *validator.WithdrawalDeleteValidator,
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
	userSessionRepo *repository.UserSessionRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	invitationRepo *repository.InvitationRepository,
	itemRepo *repository.ItemRepository,
	userStationRepo *repository.UserStationRepository,
) *DeleteAccountUsecase {
	return &DeleteAccountUsecase{
		db:                            db,
		withdrawalDeleteValidator:     withdrawalDeleteValidator,
		userRepo:                      userRepo,
		userPasswordRepo:              userPasswordRepo,
		userSessionRepo:               userSessionRepo,
		userTwoFactorAuthRepo:         userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo: userTwoFactorRecoveryCodeRepo,
		passwordResetTokenRepo:        passwordResetTokenRepo,
		emailConfirmationRepo:         emailConfirmationRepo,
		invitationRepo:                invitationRepo,
		itemRepo:                      itemRepo,
		userStationRepo:               userStationRepo,
	}
}

// DeleteAccountInput は DeleteAccountUsecase.Execute の入力。
type DeleteAccountInput struct {
	// User は退会するログイン中のユーザー。匿名にする前のメールアドレスで、そのアドレスへの確認を消す。
	User            *model.User
	CurrentPassword string
	// Confirmed は退会を元に戻せないことを確認したか。
	Confirmed bool
}

// Execute は退会のフォームを検証してから、ユーザーを退会させる。
//
// 進行中の交換・パスワードの不一致・未チェックは *model.ValidationError で返し、行には触れない。
// 既に退会していたとき (二重送信で先の送信が退会させた) は、AppErrCodeConflict の *model.AppError を返す。
//
// 検証から後始末までを1つのトランザクションで行う。
// 途中で失敗して、ログインできないのに招待だけが使える、といった半端な状態を残さないため。
//
// usersの行を最初にロックする。交換の申し込み (createTrade) と招待の作成 (replaceInvitation) も同じユーザーの行を先にロックするため、
// 退会の途中に届いた申し込みや招待は退会のコミットを待ち、退会したユーザーとの交換や、退会したユーザーの招待として作られずに終わる。
// 退会より先にコミットした申し込みは、ロックの後の進行中の交換の確認で見つける。
func (uc *DeleteAccountUsecase) Execute(ctx context.Context, input DeleteAccountInput) error {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	user, err := uc.userRepo.WithTx(tx).LockByID(ctx, input.User.ID)
	if err != nil {
		return fmt.Errorf("ユーザーのロックに失敗: %w", err)
	}
	if user == nil {
		// 先の退会がパスワードを消した後に届いた二重送信は、入力の誤りではなく退会済みとして扱う。
		return &model.AppError{Code: model.AppErrCodeConflict}
	}
	if err := uc.withdrawalDeleteValidator.WithTx(tx).Validate(ctx, validator.WithdrawalDeleteValidatorInput{
		UserID:          user.ID,
		CurrentPassword: input.CurrentPassword,
		Confirmed:       input.Confirmed,
	}); err != nil {
		return err
	}

	withdrawn, err := uc.userRepo.WithTx(tx).Withdraw(ctx, user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID))
	if err != nil {
		return fmt.Errorf("ユーザーの匿名化に失敗: %w", err)
	}
	if !withdrawn {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}
	if err := uc.userPasswordRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("パスワードの削除に失敗: %w", err)
	}
	if err := uc.userSessionRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("セッションの削除に失敗: %w", err)
	}
	if err := uc.userTwoFactorAuthRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("二要素認証の設定の削除に失敗: %w", err)
	}
	if err := uc.userTwoFactorRecoveryCodeRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("リカバリーコードの削除に失敗: %w", err)
	}
	if err := uc.passwordResetTokenRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("パスワードリセットのトークンの削除に失敗: %w", err)
	}
	if err := uc.emailConfirmationRepo.WithTx(tx).DeleteByEmail(ctx, user.Email); err != nil {
		return fmt.Errorf("メールアドレスの確認の削除に失敗: %w", err)
	}
	if err := uc.invitationRepo.WithTx(tx).RevokeUnrevokedByInviterUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("招待の取り消しに失敗: %w", err)
	}
	if err := uc.itemRepo.WithTx(tx).RemoveListedByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("リストのアイテムの取り外しに失敗: %w", err)
	}
	if err := uc.userStationRepo.WithTx(tx).DeleteByUserID(ctx, user.ID); err != nil {
		return fmt.Errorf("交換場所の削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
