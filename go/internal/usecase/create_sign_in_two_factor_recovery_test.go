package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestCreateSignInTwoFactorRecoveryUsecase_Execute はコードの1回だけの消費とセッション作成を確かめる。
func TestCreateSignInTwoFactorRecoveryUsecase_Execute(t *testing.T) {
	t.Parallel()
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	db := testutil.GetTestDB()
	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).WithEnabledAt(time.Now()).Build()
	codeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	const code = "abcd-2345"
	if err := codeRepo.CreateAll(ctx, user.ID, []string{key.RecoveryCodeDigest(auth.NormalizeRecoveryCode(code))}); err != nil {
		t.Fatalf("リカバリーコードの作成: %v", err)
	}
	uc := usecase.NewCreateSignInTwoFactorRecoveryUsecase(db, key, validator.NewSignInTwoFactorRecoveryCreateValidator(), repository.NewUserRepository(db), repository.NewUserTwoFactorAuthRepository(db), codeRepo, repository.NewUserSessionRepository(db))
	input := usecase.CreateSignInTwoFactorRecoveryInput{UserID: user.ID, Code: "ABCD - 2345", IPAddress: "203.0.113.10", UserAgent: "test"}
	output, err := uc.Execute(ctx, input)
	if err != nil || output == nil {
		t.Fatalf("Execute() = (%+v, %v)、成功を期待", output, err)
	}
	if output.User.ID != user.ID {
		t.Errorf("ログインしたユーザー = %s、期待値 = %s", output.User.ID, user.ID)
	}
	session, err := repository.NewUserSessionRepository(db).FindLiveWithUserByTokenDigest(ctx, auth.HashToken(output.Token))
	if err != nil || session == nil {
		t.Errorf("セッション = (%+v, %v)、保存済みを期待", session, err)
	}
	if count, err := codeRepo.CountUnused(ctx, user.ID); err != nil || count != 0 {
		t.Errorf("残りのコード = (%d, %v)、0を期待", count, err)
	}
	if _, err := uc.Execute(ctx, input); model.AsValidationError(err) == nil {
		t.Errorf("使用済みコードのエラー = %v、入力欄のエラーを期待", err)
	}
}

// TestCreateSignInTwoFactorRecoveryUsecase_Disabled は二要素認証が無効ならコードを使わないことを確かめる。
func TestCreateSignInTwoFactorRecoveryUsecase_Disabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testutil.GetTestDB()
	user := twoFactorUser(t)
	key := newTestTwoFactorKey(t)
	uc := usecase.NewCreateSignInTwoFactorRecoveryUsecase(db, key, validator.NewSignInTwoFactorRecoveryCreateValidator(), repository.NewUserRepository(db), repository.NewUserTwoFactorAuthRepository(db), repository.NewUserTwoFactorRecoveryCodeRepository(db), repository.NewUserSessionRepository(db))
	_, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorRecoveryInput{UserID: user.ID, Code: "abcd-2345"})
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Errorf("無効な二要素認証のエラー = %v、Conflictを期待", err)
	}
}

// TestCreateSignInTwoFactorRecoveryUsecase_SessionFailure はセッション保存失敗時にコードの消費も戻すことを確かめる。
func TestCreateSignInTwoFactorRecoveryUsecase_SessionFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testutil.GetTestDB()
	user := twoFactorUser(t)
	key := newTestTwoFactorKey(t)
	testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).WithEnabledAt(time.Now()).Build()
	codeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	const code = "bcde-2345"
	if err := codeRepo.CreateAll(ctx, user.ID, []string{key.RecoveryCodeDigest(auth.NormalizeRecoveryCode(code))}); err != nil {
		t.Fatalf("リカバリーコードの作成: %v", err)
	}
	uc := usecase.NewCreateSignInTwoFactorRecoveryUsecase(db, key, validator.NewSignInTwoFactorRecoveryCreateValidator(), repository.NewUserRepository(db), repository.NewUserTwoFactorAuthRepository(db), codeRepo, repository.NewUserSessionRepository(db))
	// PostgreSQLのtextにはNULを保存できないため、コード消費後のセッション作成だけを失敗させる。
	_, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorRecoveryInput{UserID: user.ID, Code: code, UserAgent: "\x00"})
	if err == nil {
		t.Fatal("セッション保存のエラーを期待")
	}
	if count, err := codeRepo.CountUnused(ctx, user.ID); err != nil || count != 1 {
		t.Errorf("失敗後の未使用コードの数 = (%d, %v)、1を期待", count, err)
	}
}
