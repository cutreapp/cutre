package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// UpdatePasswordUsecase は、パスワードリセットのトークンで新しいパスワードを設定する。
//
// パスワードリセットはログインしていない状態で行うため、ユーザーはセッションではなくトークンで特定する。
// トークンの使用、パスワードの置き換え、そのユーザーのセッションの全削除を1つのトランザクションで行う。
// パスワードが変わらないままトークンだけが使われることも、古いパスワードで得たセッションが残ることもないようにする。
type UpdatePasswordUsecase struct {
	db                     *sql.DB
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	passwordValidator      *validator.PasswordUpdateValidator
	userPasswordRepo       *repository.UserPasswordRepository
	userSessionRepo        *repository.UserSessionRepository
}

// NewUpdatePasswordUsecase は UpdatePasswordUsecase を生成する。
func NewUpdatePasswordUsecase(
	db *sql.DB,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	passwordValidator *validator.PasswordUpdateValidator,
	userPasswordRepo *repository.UserPasswordRepository,
	userSessionRepo *repository.UserSessionRepository,
) *UpdatePasswordUsecase {
	return &UpdatePasswordUsecase{
		db:                     db,
		passwordResetTokenRepo: passwordResetTokenRepo,
		passwordValidator:      passwordValidator,
		userPasswordRepo:       userPasswordRepo,
		userSessionRepo:        userSessionRepo,
	}
}

// UpdatePasswordInput は UpdatePasswordUsecase.Execute の入力。
type UpdatePasswordInput struct {
	// PasswordResetTokenID はCookieが運ぶ、リンクから受け取ったトークンのID。
	PasswordResetTokenID model.PasswordResetTokenID
	Password             string
}

// Execute はトークンを確かめ、パスワードを検証してから置き換える。
//
// トークンが使えないときは AppErrCodeResourceNotFound を返す。
// フォームを直して解決できる失敗ではないため、*model.ValidationError とは分ける。
// パスワードの誤りではトークンを使わず、同じリンクのまま直して送り直せる。
func (uc *UpdatePasswordUsecase) Execute(ctx context.Context, input UpdatePasswordInput) error {
	resetToken, err := uc.passwordResetTokenRepo.FindLiveByID(ctx, input.PasswordResetTokenID)
	if err != nil {
		return fmt.Errorf("パスワードリセットのトークンの取得に失敗: %w", err)
	}
	if resetToken == nil {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	if err := uc.passwordValidator.Validate(ctx, validator.PasswordUpdateValidatorInput{Password: input.Password}); err != nil {
		return err
	}

	// bcryptのコストをトランザクションの外で払い、行のロックを持つ時間を短くする。
	passwordDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		return fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	return uc.updatePassword(ctx, resetToken, passwordDigest)
}

// updatePassword はトークンを消し、パスワードを置き換え、そのユーザーのセッションを消すまでを1つのトランザクションで行う。
//
// トークンの削除は期限内の行だけを対象にする条件付きの削除にする。
// 確かめてからここまでの間に、同じトークンが使われたり、新しい申請で置き換えられたり、退会で消されたりしたときは、
// パスワードを変えずに失敗させる。
func (uc *UpdatePasswordUsecase) updatePassword(ctx context.Context, resetToken *model.PasswordResetToken, passwordDigest string) error {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	deleted, err := uc.passwordResetTokenRepo.WithTx(tx).DeleteLive(ctx, resetToken.ID)
	if err != nil {
		return fmt.Errorf("パスワードリセットのトークンの削除に失敗: %w", err)
	}
	if !deleted {
		return &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	updated, err := uc.userPasswordRepo.WithTx(tx).UpdatePasswordDigest(ctx, resetToken.UserID, passwordDigest)
	if err != nil {
		return fmt.Errorf("パスワードの更新に失敗: %w", err)
	}
	if !updated {
		return fmt.Errorf("パスワードリセットの対象ユーザーにパスワードがありません: user_id=%s", resetToken.UserID)
	}

	if err := uc.userSessionRepo.WithTx(tx).DeleteByUserID(ctx, resetToken.UserID); err != nil {
		return fmt.Errorf("セッションの削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
