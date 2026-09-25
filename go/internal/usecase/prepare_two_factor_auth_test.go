package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestPrepareTwoFactorAuthUsecase_Execute は、登録の画面を開くたびに新しい秘密鍵を作り、
// 暗号化して登録の途中の設定に保存することを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	repo := repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB())
	uc := usecase.NewPrepareTwoFactorAuthUsecase(key, repo)

	first, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{User: user})
	if err != nil || first.Setup == nil {
		t.Fatalf("1回目のExecute() = (%+v, %v)、秘密鍵を期待", first, err)
	}
	if !strings.HasPrefix(first.Setup.OTPAuthURL, "otpauth://totp/Cutre:"+user.Atname+"?") || !strings.Contains(first.Setup.OTPAuthURL, "secret="+first.Setup.Secret) {
		t.Errorf("otpauth URI = %q、アットネームと秘密鍵を含むURIを期待", first.Setup.OTPAuthURL)
	}
	second, err := uc.Execute(ctx, usecase.PrepareTwoFactorAuthInput{User: user})
	if err != nil || second.Setup == nil || second.Setup.Secret == first.Setup.Secret {
		t.Fatalf("2回目のExecute() = (%+v, %v)、1回目と別の秘密鍵を期待", second, err)
	}

	pending, err := repo.FindByUserID(ctx, user.ID)
	if err != nil || pending == nil || pending.IsEnabled() {
		t.Fatalf("二要素認証の設定 = (%+v, %v)、登録の途中の設定を期待", pending, err)
	}
	if strings.Contains(string(pending.SecretCiphertext), second.Setup.Secret) {
		t.Error("秘密鍵が平文のまま保存されている")
	}
}

// TestPrepareTwoFactorAuthUsecase_Execute_Enabled は、既に有効にしているとき、秘密鍵を差し替えずにnilを返すことを検証する。
func TestPrepareTwoFactorAuthUsecase_Execute_Enabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := twoFactorUser(t)
	testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).WithEnabledAt(time.Now()).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)

	output, err := usecase.NewPrepareTwoFactorAuthUsecase(newTestTwoFactorKey(t), repo).Execute(ctx, usecase.PrepareTwoFactorAuthInput{User: user})
	if err != nil || output.Setup != nil {
		t.Fatalf("Execute() = (%+v, %v)、秘密鍵の無い結果を期待", output, err)
	}
	enabled, err := repo.FindByUserID(ctx, user.ID)
	if err != nil || enabled == nil || string(enabled.SecretCiphertext) != "test-secret-ciphertext" {
		t.Errorf("二要素認証の設定 = (%+v, %v)、有効にした秘密鍵のままを期待", enabled, err)
	}
}
