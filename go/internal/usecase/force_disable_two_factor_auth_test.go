package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newForceDisableTwoFactorAuthUsecase はテスト用のデータベースに直接書き込む ForceDisableTwoFactorAuthUsecase を組み立てる。
func newForceDisableTwoFactorAuthUsecase() *usecase.ForceDisableTwoFactorAuthUsecase {
	db := testutil.GetTestDB()
	return usecase.NewForceDisableTwoFactorAuthUsecase(
		db,
		repository.NewUserRepository(db),
		repository.NewUserTwoFactorAuthRepository(db),
		repository.NewUserTwoFactorRecoveryCodeRepository(db),
	)
}

// TestForceDisableTwoFactorAuthUsecase_Execute は、メールアドレス・アットネーム (`@` の有無と大文字小文字を問わない) で
// ユーザーを探し、再認証なしで二要素認証の設定とリカバリーコードを削除することを検証する。
func TestForceDisableTwoFactorAuthUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	uc := newForceDisableTwoFactorAuthUsecase()

	atname := testutil.UniqueAtname()
	email := testutil.UniqueEmail("force-disable")
	byAtname := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	byEmail := testutil.NewUserBuilder(t, db).WithEmail(email).Build()

	for identifier, userID := range map[string]model.UserID{
		"@" + strings.ToUpper(atname): byAtname,
		" " + email + " ":             byEmail,
	} {
		testutil.NewUserTwoFactorAuthBuilder(t, db, userID).WithEnabledAt(time.Now()).Build()
		if err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CreateAll(ctx, userID, []string{"digest-1"}); err != nil {
			t.Fatalf("リカバリーコードの作成のエラー = %v", err)
		}

		output, err := uc.Execute(ctx, usecase.ForceDisableTwoFactorAuthInput{Identifier: identifier})
		if err != nil {
			t.Fatalf("%q: Execute()のエラー = %v", identifier, err)
		}
		if output.User.ID != userID || !output.Disabled {
			t.Errorf("%q: 結果 = (%s, %t)、(%s, true) を期待", identifier, output.User.ID, output.Disabled, userID)
		}
		assertTwoFactorAuthRemoved(t, userID)
	}
}

// TestForceDisableTwoFactorAuthUsecase_Execute_NotEnabled は、有効にしていないユーザーでは Disabled をfalseにし、
// 登録の途中の設定は消すことを検証する。
func TestForceDisableTwoFactorAuthUsecase_Execute_NotEnabled(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	atname := testutil.UniqueAtname()
	userID := testutil.NewUserBuilder(t, db).WithAtname(atname).Build()
	testutil.NewUserTwoFactorAuthBuilder(t, db, userID).Build()

	output, err := newForceDisableTwoFactorAuthUsecase().Execute(context.Background(), usecase.ForceDisableTwoFactorAuthInput{Identifier: atname})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Disabled {
		t.Error("Disabled = true、有効にしていないユーザーではfalseを期待")
	}
	assertTwoFactorAuthRemoved(t, userID)
}

// TestForceDisableTwoFactorAuthUsecase_Execute_NotFound は、見つからないユーザー・退会したユーザー・空の指定で
// AppErrCodeResourceNotFound を返すことを検証する。
func TestForceDisableTwoFactorAuthUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	withdrawnAtname := testutil.UniqueAtname()
	testutil.NewUserBuilder(t, db).WithAtname(withdrawnAtname).WithDeletedAt(time.Now()).Build()

	for _, identifier := range []string{testutil.UniqueAtname(), testutil.UniqueEmail("missing"), withdrawnAtname, "@", ""} {
		_, err := newForceDisableTwoFactorAuthUsecase().Execute(context.Background(), usecase.ForceDisableTwoFactorAuthInput{Identifier: identifier})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%q: エラー = %v、AppErrCodeResourceNotFoundを期待", identifier, err)
		}
	}
}
