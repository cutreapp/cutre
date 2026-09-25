package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
)

// ForceDisableTwoFactorAuthUsecase は、再認証を求めずにユーザーの二要素認証を無効にする。
//
// 認証アプリとリカバリーコードを両方失った人を、管理者がCLIから救済するのに使う。
// 本人確認は管理者がCLIの外で済ませる前提のため、Webの画面からは呼ばない。
type ForceDisableTwoFactorAuthUsecase struct {
	db                            *sql.DB
	userRepo                      *repository.UserRepository
	userTwoFactorAuthRepo         *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository
}

// NewForceDisableTwoFactorAuthUsecase は ForceDisableTwoFactorAuthUsecase を生成する。
func NewForceDisableTwoFactorAuthUsecase(
	db *sql.DB,
	userRepo *repository.UserRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
) *ForceDisableTwoFactorAuthUsecase {
	return &ForceDisableTwoFactorAuthUsecase{
		db:                            db,
		userRepo:                      userRepo,
		userTwoFactorAuthRepo:         userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo: userTwoFactorRecoveryCodeRepo,
	}
}

// ForceDisableTwoFactorAuthInput は ForceDisableTwoFactorAuthUsecase.Execute の入力。
type ForceDisableTwoFactorAuthInput struct {
	// Identifier はユーザーのメールアドレス、またはアットネーム (先頭の `@` は有っても無くてもよい)。
	Identifier string
}

// ForceDisableTwoFactorAuthOutput は ForceDisableTwoFactorAuthUsecase.Execute の結果。
type ForceDisableTwoFactorAuthOutput struct {
	User *model.User
	// Disabled は有効にしていた二要素認証を無効にしたか。有効にしていなかったときはfalse。
	Disabled bool
}

// Execute はユーザーを探し、二要素認証の設定とリカバリーコードを1つのトランザクションで削除する。
//
// 途中で `@` を含む入力はメールアドレスとして、それ以外はアットネームとして探す。アットネームは `@` を含まないため。
// ユーザーが見つからない (退会したユーザーを含む) ときは AppErrCodeResourceNotFound の *model.AppError を返す。
// 有効にしていなかったときも、登録の途中の設定が残っていれば消し、Disabled をfalseにして返す。
func (uc *ForceDisableTwoFactorAuthUsecase) Execute(ctx context.Context, input ForceDisableTwoFactorAuthInput) (*ForceDisableTwoFactorAuthOutput, error) {
	user, err := uc.findUser(ctx, strings.TrimSpace(input.Identifier))
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, &model.AppError{Code: model.AppErrCodeResourceNotFound}
	}

	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := deleteTwoFactorAuth(ctx, tx, uc.userTwoFactorAuthRepo, uc.userTwoFactorRecoveryCodeRepo, user.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &ForceDisableTwoFactorAuthOutput{User: user, Disabled: twoFactorAuth != nil && twoFactorAuth.IsEnabled()}, nil
}

// findUser はメールアドレスまたはアットネームでユーザーを探す。見つからないときは (nil, nil) を返す。
func (uc *ForceDisableTwoFactorAuthUsecase) findUser(ctx context.Context, identifier string) (*model.User, error) {
	atname := strings.TrimPrefix(identifier, "@")
	if atname == "" {
		return nil, nil
	}
	if strings.Contains(atname, "@") {
		return uc.userRepo.FindByEmail(ctx, identifier)
	}

	return uc.userRepo.FindByAtname(ctx, atname)
}
