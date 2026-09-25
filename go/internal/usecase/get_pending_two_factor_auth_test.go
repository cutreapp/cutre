package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetPendingTwoFactorAuthUsecase_Execute は、登録の途中の秘密鍵を、登録の画面に出したものと同じ形で返すことを検証する。
func TestGetPendingTwoFactorAuthUsecase_Execute(t *testing.T) {
	t.Parallel()

	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	secret := prepareTwoFactorAuth(t, key, user)

	uc := usecase.NewGetPendingTwoFactorAuthUsecase(key, repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()))
	output, err := uc.Execute(context.Background(), usecase.GetPendingTwoFactorAuthInput{User: user})
	if err != nil || output.Setup == nil {
		t.Fatalf("Execute() = (%+v, %v)、秘密鍵を期待", output, err)
	}
	if output.Setup.Secret != secret || output.Setup.OTPAuthURL == "" {
		t.Errorf("秘密鍵 = %+v、登録の画面に出した %q とotpauth URIを期待", output.Setup, secret)
	}
}

// TestGetPendingTwoFactorAuthUsecase_Execute_NotPending は、登録の途中の設定が無いときにnilを返すことを検証する。
// 有効にした設定の秘密鍵は、もう画面に出さない。
func TestGetPendingTwoFactorAuthUsecase_Execute_NotPending(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	uc := usecase.NewGetPendingTwoFactorAuthUsecase(newTestTwoFactorKey(t), repository.NewUserTwoFactorAuthRepository(db))
	withoutSetting := twoFactorUser(t)
	enabled := twoFactorUser(t)
	testutil.NewUserTwoFactorAuthBuilder(t, db, enabled.ID).WithEnabledAt(time.Now()).Build()

	for name, input := range map[string]usecase.GetPendingTwoFactorAuthInput{
		"設定が無い": {User: withoutSetting},
		"有効にした": {User: enabled},
	} {
		output, err := uc.Execute(context.Background(), input)
		if err != nil || output.Setup != nil {
			t.Errorf("%s: Execute() = (%+v, %v)、秘密鍵の無い結果を期待", name, output, err)
		}
	}
}
