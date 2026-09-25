package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newCreateSignInTwoFactorUsecase はテスト用のデータベースに直接書き込む CreateSignInTwoFactorUsecase を組み立てる。
func newCreateSignInTwoFactorUsecase(key *auth.TwoFactorKey) *usecase.CreateSignInTwoFactorUsecase {
	db := testutil.GetTestDB()
	return usecase.NewCreateSignInTwoFactorUsecase(
		key,
		validator.NewSignInTwoFactorCreateValidator(),
		repository.NewUserRepository(db),
		repository.NewUserTwoFactorAuthRepository(db),
	)
}

// enableTwoFactorAuth は、ユーザーに秘密鍵を暗号化した有効な二要素認証の設定を作り、その秘密鍵を返す。
func enableTwoFactorAuth(t *testing.T, key *auth.TwoFactorKey, userID model.UserID) string {
	t.Helper()

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("秘密鍵の生成のエラー = %v", err)
	}
	id := uuid.UUID(userID)
	ciphertext, err := key.EncryptTOTPSecret(secret, id[:])
	if err != nil {
		t.Fatalf("秘密鍵の暗号化のエラー = %v", err)
	}
	testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), userID).
		WithSecretCiphertext(ciphertext).
		WithEnabledAt(time.Now()).
		Build()

	return secret
}

// TestCreateSignInTwoFactorUsecase_Execute は、今のコードでユーザーを返し、同じコードの2回目と誤ったコードを
// コードの欄のエラーにすることを検証する。
func TestCreateSignInTwoFactorUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newCreateSignInTwoFactorUsecase(key)
	user := twoFactorUser(t)
	code := currentTOTPCode(t, enableTwoFactorAuth(t, key, user.ID))

	// 貼り付けで混じった空白とハイフンは取り除いて照合する。
	output, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: user.ID, Code: code[:3] + " - " + code[3:]})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.User.ID != user.ID {
		t.Errorf("UserID = %s、期待値 = %s", output.User.ID, user.ID)
	}

	// 同じタイムステップのコードは、盗み見られたコードの使い回しを防ぐため2回目を受け付けない。
	_, err = uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: user.ID, Code: code})
	assertCodeFieldError(t, err, "コードが正しくありません。認証アプリに表示されている最新のコードを入力してください")

	wrongCode := "000000"
	if wrongCode == code {
		wrongCode = "111111"
	}
	_, err = uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: user.ID, Code: wrongCode})
	assertCodeFieldError(t, err, "コードが正しくありません。認証アプリに表示されている最新のコードを入力してください")

	_, err = uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: user.ID, Code: "12345"})
	assertCodeFieldError(t, err, "6桁の数字で入力してください")
}

// TestCreateSignInTwoFactorUsecase_Execute_Conflict は、パスワードを確かめた後に二要素認証が有効でなくなった・
// ユーザーが退会したときに、コードを照合せず AppErrCodeConflict を返すことを検証する。
func TestCreateSignInTwoFactorUsecase_Execute_Conflict(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newCreateSignInTwoFactorUsecase(key)
	db := testutil.GetTestDB()

	withoutTwoFactorAuth := twoFactorUser(t)
	pending := twoFactorUser(t)
	testutil.NewUserTwoFactorAuthBuilder(t, db, pending.ID).Build()
	withdrawn := testutil.NewUserBuilder(t, db).WithDeletedAt(time.Now()).Build()

	for name, userID := range map[string]model.UserID{
		"二要素認証の設定が無い":  withoutTwoFactorAuth.ID,
		"認証アプリへの登録の途中": pending.ID,
		"退会した":         withdrawn,
	} {
		_, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: userID, Code: "123456"})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
			t.Errorf("%s: エラー = %v、AppErrCodeConflictを期待", name, err)
		}
	}
}

// assertCodeFieldError は、err がコードの欄に want の1件だけを持つ ValidationError であることを確かめる。
func assertCodeFieldError(t *testing.T, err error, want string) {
	t.Helper()

	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("エラー = %v、ValidationErrorを期待", err)
	}
	if got := ve.GetFieldErrors("code"); len(got) != 1 || got[0] != want {
		t.Errorf("codeのエラー = %v、期待値 = [%s]", got, want)
	}
}
