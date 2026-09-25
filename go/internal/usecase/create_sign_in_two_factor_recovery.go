package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// CreateSignInTwoFactorRecoveryUsecase は、パスワードを確かめたユーザーのリカバリーコードを消費し、セッションを作る。
//
// 認証アプリのコード (CreateSignInTwoFactorUsecase と CreateSessionUsecase の2段) と違い、消費したコードは戻せない。
// そのため消費とセッションの作成を同じトランザクションで行い、セッションの作成に失敗したらコードの消費も取り消す。
// コードを失ったままログインできない状態を作らないため。
type CreateSignInTwoFactorRecoveryUsecase struct {
	db                               *sql.DB
	twoFactorKey                     *auth.TwoFactorKey
	signInTwoFactorRecoveryValidator *validator.SignInTwoFactorRecoveryCreateValidator
	userRepo                         *repository.UserRepository
	userTwoFactorAuthRepo            *repository.UserTwoFactorAuthRepository
	userTwoFactorRecoveryCodeRepo    *repository.UserTwoFactorRecoveryCodeRepository
	userSessionRepo                  *repository.UserSessionRepository
}

// NewCreateSignInTwoFactorRecoveryUsecase は CreateSignInTwoFactorRecoveryUsecase を生成する。
func NewCreateSignInTwoFactorRecoveryUsecase(
	db *sql.DB,
	twoFactorKey *auth.TwoFactorKey,
	signInTwoFactorRecoveryValidator *validator.SignInTwoFactorRecoveryCreateValidator,
	userRepo *repository.UserRepository,
	userTwoFactorAuthRepo *repository.UserTwoFactorAuthRepository,
	userTwoFactorRecoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository,
	userSessionRepo *repository.UserSessionRepository,
) *CreateSignInTwoFactorRecoveryUsecase {
	return &CreateSignInTwoFactorRecoveryUsecase{
		db:                               db,
		twoFactorKey:                     twoFactorKey,
		signInTwoFactorRecoveryValidator: signInTwoFactorRecoveryValidator,
		userRepo:                         userRepo,
		userTwoFactorAuthRepo:            userTwoFactorAuthRepo,
		userTwoFactorRecoveryCodeRepo:    userTwoFactorRecoveryCodeRepo,
		userSessionRepo:                  userSessionRepo,
	}
}

// CreateSignInTwoFactorRecoveryInput は CreateSignInTwoFactorRecoveryUsecase.Execute の入力。
type CreateSignInTwoFactorRecoveryInput struct {
	// UserID はパスワードを確かめたユーザーのID。署名付きのCookieから読んだ値を渡す。
	UserID model.UserID
	// Code は入力されたままのコード。大文字小文字・ハイフン・空白・全角の文字はここで正規化する。
	Code string
	// IPAddress と UserAgent は、セッションをどこから始めたかの記録として残す。
	IPAddress string
	UserAgent string
}

// CreateSignInTwoFactorRecoveryOutput は CreateSignInTwoFactorRecoveryUsecase.Execute の結果。
type CreateSignInTwoFactorRecoveryOutput struct {
	// User はコードが合ったユーザー。
	User *model.User
	// Token はセッションCookieに書き込む平文のトークン。
	Token string
}

// Execute はコードを照合し、合っていればコードを使用済みにしてセッションを作る。
//
// 形式の誤り・不一致・使用済みのコードは、コードの欄の *model.ValidationError で返す。
// ユーザーが退会した・二要素認証を無効にしたときは、パスワードの確認からやり直させるため AppErrCodeConflict を返す。
func (uc *CreateSignInTwoFactorRecoveryUsecase) Execute(ctx context.Context, input CreateSignInTwoFactorRecoveryInput) (*CreateSignInTwoFactorRecoveryOutput, error) {
	code := auth.NormalizeRecoveryCode(input.Code)
	if err := uc.signInTwoFactorRecoveryValidator.Validate(ctx, validator.SignInTwoFactorRecoveryCreateValidatorInput{Code: code}); err != nil {
		return nil, err
	}

	user, err := uc.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("ユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, &model.AppError{Code: model.AppErrCodeConflict}
	}

	twoFactorAuth, err := uc.userTwoFactorAuthRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("二要素認証の設定の取得に失敗: %w", err)
	}
	if twoFactorAuth == nil || !twoFactorAuth.IsEnabled() {
		return nil, &model.AppError{Code: model.AppErrCodeConflict}
	}

	tx, err := uc.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 未使用のコードだけを条件付きのUPDATEで使用済みにし、同じコードの同時の送信を二重に通さない。
	used, err := uc.userTwoFactorRecoveryCodeRepo.WithTx(tx).Use(ctx, user.ID, uc.twoFactorKey.RecoveryCodeDigest(code))
	if err != nil {
		return nil, fmt.Errorf("リカバリーコードの消費に失敗: %w", err)
	}
	if !used {
		ve := model.NewValidationError()
		ve.AddField("code", i18n.T(ctx, "validation_recovery_code_incorrect"))
		return nil, ve
	}

	token, err := createUserSession(ctx, uc.userSessionRepo.WithTx(tx), CreateSessionInput{
		UserID:    user.ID,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateSignInTwoFactorRecoveryOutput{User: user, Token: token}, nil
}
