package usecase

import (
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

// EnableTwoFactorAuthUsecase は、認証アプリのコードを照合して二要素認証を有効にし、リカバリーコードを発行する。
//
// リカバリーコードは平文を返すだけで保存しない (ダイジェストだけを保存する)。画面に出せるのはこの一度きりになる。
type EnableTwoFactorAuthUsecase struct {
	db                            *sql.DB
	twoFactorKey                  *auth.TwoFactorKey
	twoFactorAuthValidator        *validator.TwoFactorAuthCreateValidator
	userTwoFactorAuthRepo         *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository
	// beforeEnable は照合の後、有効にする前に呼ぶ関数。本番ではnilで、テストが照合後の秘密鍵の差し替えを再現するのに使う。
	beforeEnable func(ctx context.Context)
}

// NewEnableTwoFactorAuthUsecase は EnableTwoFactorAuthUsecase を生成する。
func NewEnableTwoFactorAuthUsecase(
	db *sql.DB,
	twoFactorKey *auth.TwoFactorKey,
	twoFactorAuthValidator *validator.TwoFactorAuthCreateValidator,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
) *EnableTwoFactorAuthUsecase {
	return &EnableTwoFactorAuthUsecase{
		db:                            db,
		twoFactorKey:                  twoFactorKey,
		twoFactorAuthValidator:        twoFactorAuthValidator,
		userTwoFactorAuthRepo:         userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo: userTwoFactorRecoveryCodeRepo,
	}
}

// EnableTwoFactorAuthInput は EnableTwoFactorAuthUsecase.Execute の入力。
type EnableTwoFactorAuthInput struct {
	UserID model.UserID
	// Code は入力されたままのコード。空白・ハイフン・全角の数字はここで正規化する。
	Code string
}

// EnableTwoFactorAuthOutput は EnableTwoFactorAuthUsecase.Execute の結果。
type EnableTwoFactorAuthOutput struct {
	// RecoveryCodes は発行したリカバリーコードの平文 (`xxxx-xxxx` の形)。
	RecoveryCodes []string
}

// Execute はコードを照合し、二要素認証の有効化とリカバリーコードの作成を1つのトランザクションで行う。
//
// 形式の誤りと照合の失敗は、コードの欄の *model.ValidationError で返す。
// 登録の途中の設定が無いとき (既に有効にした・画面を開いていない・照合の後に別の画面で秘密鍵が差し替わった) は、
// AppErrCodeConflict の *model.AppError を返す。
func (uc *EnableTwoFactorAuthUsecase) Execute(ctx context.Context, input EnableTwoFactorAuthInput) (*EnableTwoFactorAuthOutput, error) {
	code := auth.NormalizeTOTPCode(input.Code)
	if err := uc.twoFactorAuthValidator.Validate(ctx, validator.TwoFactorAuthCreateValidatorInput{Code: code}); err != nil {
		return nil, err
	}

	pending, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if pending == nil || pending.IsEnabled() {
		return nil, &model.AppError{Code: model.AppErrCodeConflict}
	}

	secret, err := decryptTwoFactorSecret(uc.twoFactorKey, pending)
	if err != nil {
		return nil, err
	}
	step, ok := auth.MatchTOTPCode(secret, code, time.Now())
	if !ok {
		ve := model.NewValidationError()
		ve.AddField("code", i18n.T(ctx, "validation_totp_code_incorrect"))
		return nil, ve
	}

	recoveryCodes, err := auth.GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("リカバリーコードの生成に失敗: %w", err)
	}
	digests := make([]string, len(recoveryCodes))
	for i, recoveryCode := range recoveryCodes {
		digests[i] = uc.twoFactorKey.RecoveryCodeDigest(auth.NormalizeRecoveryCode(recoveryCode))
	}

	if uc.beforeEnable != nil {
		uc.beforeEnable(ctx)
	}
	if err := uc.enable(ctx, pending, step, digests); err != nil {
		return nil, err
	}

	return &EnableTwoFactorAuthOutput{RecoveryCodes: recoveryCodes}, nil
}

// enable は照合した秘密鍵の設定を有効にし、リカバリーコードを作り直す。
//
// 有効にできなかったとき (同時に送られた別の要求が先に有効にした・秘密鍵が差し替わった) は、リカバリーコードも作らない。
// 残っていたリカバリーコードを消してから作るのは、有効な設定と対になるコードだけを残すため。
func (uc *EnableTwoFactorAuthUsecase) enable(ctx context.Context, pending *model.UserTwoFactorAuth, step int64, digests []string) error {
	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	enabled, err := uc.userTwoFactorAuthRepo.WithTx(tx).Enable(ctx, pending.UserID, pending.SecretCiphertext, step)
	if err != nil {
		return fmt.Errorf("二要素認証の有効化に失敗: %w", err)
	}
	if !enabled {
		return &model.AppError{Code: model.AppErrCodeConflict}
	}

	recoveryCodeRepo := uc.userTwoFactorRecoveryCodeRepo.WithTx(tx)
	if err := recoveryCodeRepo.DeleteByUserID(ctx, pending.UserID); err != nil {
		return fmt.Errorf("残っていたリカバリーコードの削除に失敗: %w", err)
	}
	if err := recoveryCodeRepo.CreateAll(ctx, pending.UserID, digests); err != nil {
		return fmt.Errorf("リカバリーコードの作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
