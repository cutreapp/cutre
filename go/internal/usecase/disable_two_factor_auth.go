package usecase

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// DisableTwoFactorAuthUsecase は、パスワードまたは認証アプリのコードで再認証し、二要素認証を無効にする。
//
// 無効にしてもセッションはそのまま残す。第2の要素を外すだけで、ログインし直す理由は無いため。
type DisableTwoFactorAuthUsecase struct {
	db                            *sql.DB
	twoFactorKey                  *auth.TwoFactorKey
	twoFactorAuthDeleteValidator  *validator.TwoFactorAuthDeleteValidator
	userPasswordRepo              *repository.UserPasswordRepository
	userTwoFactorAuthRepo         *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository
	// beforeDisable は再認証の後、削除の前に呼ぶテスト用のフック。本番ではnil。
	beforeDisable func(ctx context.Context)
}

// NewDisableTwoFactorAuthUsecase は DisableTwoFactorAuthUsecase を生成する。
func NewDisableTwoFactorAuthUsecase(
	db *sql.DB,
	twoFactorKey *auth.TwoFactorKey,
	twoFactorAuthDeleteValidator *validator.TwoFactorAuthDeleteValidator,
	userPasswordRepo *repository.UserPasswordRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
) *DisableTwoFactorAuthUsecase {
	return &DisableTwoFactorAuthUsecase{
		db:                            db,
		twoFactorKey:                  twoFactorKey,
		twoFactorAuthDeleteValidator:  twoFactorAuthDeleteValidator,
		userPasswordRepo:              userPasswordRepo,
		userTwoFactorAuthRepo:         userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo: userTwoFactorRecoveryCodeRepo,
	}
}

// DisableTwoFactorAuthInput は DisableTwoFactorAuthUsecase.Execute の入力。
type DisableTwoFactorAuthInput struct {
	UserID model.UserID
	// Credential は入力されたままのパスワードまたは認証アプリのコード。
	Credential string
}

// Execute は再認証してから、二要素認証の設定とリカバリーコードを1つのトランザクションで削除する。
//
// 入力は、正規化して6桁の数字になり今のコードと合えば認証アプリのコードとして、そうでなければパスワードとして照合する。
// パスワードは8文字以上のため、6桁の数字は通常パスワードになり得ない。
// それでも区切りの文字を含むパスワードが6桁の数字に正規化されることはあるため、コードとして合わなければパスワードとしても照合する。
//
// 未入力と照合の失敗は、入力の欄の *model.ValidationError で返す。どちらとして照合して失敗したかは明かさない。
// 二要素認証を有効にしていないとき (別の画面で既に無効にした・登録の途中) や、再認証後に設定が入れ替わったときは、
// AppErrCodeConflict の *model.AppError を返す。
func (uc *DisableTwoFactorAuthUsecase) Execute(ctx context.Context, input DisableTwoFactorAuthInput) error {
	if err := uc.twoFactorAuthDeleteValidator.Validate(ctx, validator.TwoFactorAuthDeleteValidatorInput{Credential: input.Credential}); err != nil {
		return err
	}

	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if twoFactorAuth == nil || !twoFactorAuth.IsEnabled() {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}

	secret, err := decryptTwoFactorSecret(uc.twoFactorKey, twoFactorAuth)
	if err != nil {
		return err
	}
	step, totpMatched := auth.MatchTOTPCode(secret, auth.NormalizeTOTPCode(input.Credential), time.Now())
	if !totpMatched {
		passwordMatched, err := uc.checkPassword(ctx, input.UserID, input.Credential)
		if err != nil {
			return err
		}
		if !passwordMatched {
			return reauthIncorrectError(ctx)
		}
	}
	if uc.beforeDisable != nil {
		uc.beforeDisable(ctx)
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if totpMatched {
		// ログインで使ったばかりのコードを使い回されないよう、ログインと同じく使ったステップを条件付きで記録する。
		used, err := uc.userTwoFactorAuthRepo.WithTx(tx).UseMatchingStep(ctx, twoFactorAuth, step)
		if err != nil {
			return fmt.Errorf("使ったタイムステップの記録に失敗: %w", err)
		}
		if !used {
			current, err := uc.userTwoFactorAuthRepo.WithTx(tx).FindByUserID(ctx, input.UserID)
			if err != nil {
				return fmt.Errorf("二要素認証の設定の再取得に失敗: %w", err)
			}
			if !sameTwoFactorAuthSetting(current, twoFactorAuth) {
				return &model.AppError{Code: model.AppErrCodeConflict}
			}
			return reauthIncorrectError(ctx)
		}
	}
	deleted, err := uc.userTwoFactorAuthRepo.WithTx(tx).DeleteMatching(ctx, twoFactorAuth)
	if err != nil {
		return fmt.Errorf("二要素認証の設定の削除に失敗: %w", err)
	}
	if !deleted {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}
	if err := uc.userTwoFactorRecoveryCodeRepo.WithTx(tx).DeleteByUserID(ctx, input.UserID); err != nil {
		return fmt.Errorf("リカバリーコードの削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}

// sameTwoFactorAuthSetting は再認証に使った有効な設定が残っているかを返す。
func sameTwoFactorAuthSetting(current, expected *model.UserTwoFactorAuth) bool {
	return current != nil && current.IsEnabled() && current.ID == expected.ID && bytes.Equal(current.SecretCiphertext, expected.SecretCiphertext)
}

// checkPassword は入力がユーザーの今のパスワードと一致するかを返す。
func (uc *DisableTwoFactorAuthUsecase) checkPassword(ctx context.Context, userID model.UserID, plainPassword string) (bool, error) {
	password, err := uc.userPasswordRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("パスワードの取得に失敗: %w", err)
	}
	if password == nil {
		return false, nil
	}

	return auth.CheckPassword(password.PasswordDigest, plainPassword) == nil, nil
}

// reauthIncorrectError は再認証の入力が合わなかったことを、入力の欄のエラーで返す。
func reauthIncorrectError(ctx context.Context) *model.ValidationError {
	ve := model.NewValidationError()
	ve.AddField("credential", i18n.T(ctx, "validation_two_factor_auth_reauth_incorrect"))

	return ve
}

// deleteTwoFactorAuth はtx内で、ユーザーの二要素認証の設定とリカバリーコードを削除する。
// 無効にしたアカウントにリカバリーコードだけが残らないよう、設定と一緒に消す。
func deleteTwoFactorAuth(
	ctx context.Context,
	tx *sql.Tx,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
	userID model.UserID,
) error {
	if err := userTwoFactorAuthRepo.WithTx(tx).DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("二要素認証の設定の削除に失敗: %w", err)
	}
	if err := userTwoFactorRecoveryCodeRepo.WithTx(tx).DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("リカバリーコードの削除に失敗: %w", err)
	}

	return nil
}
