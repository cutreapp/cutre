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

// newDisableTwoFactorAuthUsecase はテスト用のデータベースに直接書き込む DisableTwoFactorAuthUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newDisableTwoFactorAuthUsecase(key *auth.TwoFactorKey) *usecase.DisableTwoFactorAuthUsecase {
	db := testutil.GetTestDB()
	return usecase.NewDisableTwoFactorAuthUsecase(
		db,
		key,
		validator.NewTwoFactorAuthDeleteValidator(),
		repository.NewUserPasswordRepository(db),
		repository.NewUserTwoFactorAuthRepository(db),
		repository.NewUserTwoFactorRecoveryCodeRepository(db),
	)
}

// enabledTwoFactorUser は、パスワードと有効な二要素認証の設定・リカバリーコードを持つユーザーを作り、ユーザーと秘密鍵を返す。
func enabledTwoFactorUser(t *testing.T, key *auth.TwoFactorKey, password string) (*model.User, string) {
	t.Helper()

	user := twoFactorUser(t)
	testutil.NewUserPasswordBuilder(t, testutil.GetTestDB()).WithUserID(user.ID).WithPassword(password).Build()
	secret := enableTwoFactorAuth(t, key, user.ID)
	if err := repository.NewUserTwoFactorRecoveryCodeRepository(testutil.GetTestDB()).CreateAll(context.Background(), user.ID, []string{"digest-1", "digest-2"}); err != nil {
		t.Fatalf("リカバリーコードの作成のエラー = %v", err)
	}

	return user, secret
}

// assertTwoFactorAuthRemoved は、ユーザーの二要素認証の設定とリカバリーコードが残っていないことを確かめる。
func assertTwoFactorAuthRemoved(t *testing.T, userID model.UserID) {
	t.Helper()

	ctx := context.Background()
	db := testutil.GetTestDB()
	if setting, err := repository.NewUserTwoFactorAuthRepository(db).FindByUserID(ctx, userID); err != nil || setting != nil {
		t.Errorf("二要素認証の設定 = (%+v, %v)、(nil, nil) を期待", setting, err)
	}
	if count, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CountUnused(ctx, userID); err != nil || count != 0 {
		t.Errorf("リカバリーコードの数 = (%d, %v)、(0, nil) を期待", count, err)
	}
}

// assertCredentialFieldError は、err が再認証の欄に want の1件だけを持つ ValidationError であることを確かめる。
func assertCredentialFieldError(t *testing.T, err error, want string) {
	t.Helper()

	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("エラー = %v、ValidationErrorを期待", err)
	}
	if got := ve.GetFieldErrors("credential"); len(got) != 1 || got[0] != want {
		t.Errorf("credentialのエラー = %v、期待値 = [%s]", got, want)
	}
}

// TestDisableTwoFactorAuthUsecase_Execute は、今のパスワードか認証アプリのコードで再認証できれば、
// 二要素認証の設定とリカバリーコードを削除することを検証する。コードは貼り付けで混じった区切りを取り除いて照合する。
func TestDisableTwoFactorAuthUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newDisableTwoFactorAuthUsecase(key)

	byPassword, _ := enabledTwoFactorUser(t, key, "correct horse battery")
	if err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: byPassword.ID, Credential: "correct horse battery"}); err != nil {
		t.Fatalf("パスワードでのExecute()のエラー = %v", err)
	}
	assertTwoFactorAuthRemoved(t, byPassword.ID)

	byCode, secret := enabledTwoFactorUser(t, key, "correct horse battery")
	code := currentTOTPCode(t, secret)
	if err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: byCode.ID, Credential: code[:3] + " " + code[3:]}); err != nil {
		t.Fatalf("コードでのExecute()のエラー = %v", err)
	}
	assertTwoFactorAuthRemoved(t, byCode.ID)
}

// TestDisableTwoFactorAuthUsecase_Execute_PasswordShapedLikeCode は、区切りを除くと6桁の数字になるパスワードでも、
// コードとして合わなければパスワードとして照合して無効にできることを検証する。
func TestDisableTwoFactorAuthUsecase_Execute_PasswordShapedLikeCode(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newDisableTwoFactorAuthUsecase(key)
	user, _ := enabledTwoFactorUser(t, key, "12 34 56")

	if err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: user.ID, Credential: "12 34 56"}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	assertTwoFactorAuthRemoved(t, user.ID)
}

// TestDisableTwoFactorAuthUsecase_Execute_Rejected は、未入力・誤ったパスワード・誤ったコード・ログインで使ったコードを
// 入力の欄のエラーにし、二要素認証を残すことを検証する。
func TestDisableTwoFactorAuthUsecase_Execute_Rejected(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newDisableTwoFactorAuthUsecase(key)
	user, secret := enabledTwoFactorUser(t, key, "correct horse battery")

	// ログインで使ったコードは、使い回しを防ぐため再認証にも使えない。
	usedCode := currentTOTPCode(t, secret)
	if _, err := newCreateSignInTwoFactorUsecase(key).Execute(ctx, usecase.CreateSignInTwoFactorInput{UserID: user.ID, Code: usedCode}); err != nil {
		t.Fatalf("ログインでのコードの使用のエラー = %v", err)
	}
	wrongCode := "000000"
	if wrongCode == usedCode {
		wrongCode = "111111"
	}

	for credential, want := range map[string]string{
		"":                  "入力してください",
		"wrong password":    "パスワードまたはコードが正しくありません",
		wrongCode:           "パスワードまたはコードが正しくありません",
		usedCode:            "パスワードまたはコードが正しくありません",
		"correct horse bat": "パスワードまたはコードが正しくありません",
	} {
		err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: user.ID, Credential: credential})
		assertCredentialFieldError(t, err, want)
	}

	status, err := usecase.NewGetTwoFactorAuthStatusUsecase(
		repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()),
		repository.NewUserTwoFactorRecoveryCodeRepository(testutil.GetTestDB()),
	).Execute(ctx, usecase.GetTwoFactorAuthStatusInput{UserID: user.ID})
	if err != nil {
		t.Fatalf("状態の取得のエラー = %v", err)
	}
	if status.TwoFactorAuth == nil || status.UnusedRecoveryCodeCount != 2 {
		t.Errorf("状態 = %+v、有効でリカバリーコードが2件残っていることを期待", status)
	}
}

// TestDisableTwoFactorAuthUsecase_Execute_Conflict は、二要素認証を有効にしていないとき (設定が無い・登録の途中) に、
// 再認証せず AppErrCodeConflict を返すことを検証する。
func TestDisableTwoFactorAuthUsecase_Execute_Conflict(t *testing.T) {
	t.Parallel()

	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	key := newTestTwoFactorKey(t)
	uc := newDisableTwoFactorAuthUsecase(key)

	withoutTwoFactorAuth := twoFactorUser(t)
	pending := twoFactorUser(t)
	testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), pending.ID).Build()

	for name, userID := range map[string]model.UserID{
		"二要素認証の設定が無い":  withoutTwoFactorAuth.ID,
		"認証アプリへの登録の途中": pending.ID,
	} {
		err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: userID, Credential: testutil.DefaultBuilderPassword})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
			t.Errorf("%s: エラー = %v、AppErrCodeConflictを期待", name, err)
		}
	}
}

// TestDisableTwoFactorAuthUsecase_Execute_SettingReplaced は、再認証の直後に設定が作り直された場合、
// 古いパスワードやコードの照合結果で新しい設定とリカバリーコードを削除しないことを検証する。
func TestDisableTwoFactorAuthUsecase_Execute_SettingReplaced(t *testing.T) {
	t.Parallel()

	for _, credentialKind := range []string{"パスワード", "認証アプリのコード"} {
		t.Run(credentialKind, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
			db := testutil.GetTestDB()
			key := newTestTwoFactorKey(t)
			uc := newDisableTwoFactorAuthUsecase(key)
			user, oldSecret := enabledTwoFactorUser(t, key, testutil.DefaultBuilderPassword)
			credential := testutil.DefaultBuilderPassword
			if credentialKind == "認証アプリのコード" {
				credential = currentTOTPCode(t, oldSecret)
			}

			var newSettingID model.UserTwoFactorAuthID
			uc.SetBeforeDisableHook(func(ctx context.Context) {
				settingRepo := repository.NewUserTwoFactorAuthRepository(db)
				recoveryRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
				if err := settingRepo.DeleteByUserID(ctx, user.ID); err != nil {
					t.Fatalf("古い設定の削除のエラー = %v", err)
				}
				if err := recoveryRepo.DeleteByUserID(ctx, user.ID); err != nil {
					t.Fatalf("古いリカバリーコードの削除のエラー = %v", err)
				}
				newSecret, err := auth.GenerateTOTPSecret()
				if err != nil {
					t.Fatalf("新しい秘密鍵の生成のエラー = %v", err)
				}
				id := uuid.UUID(user.ID)
				ciphertext, err := key.EncryptTOTPSecret(newSecret, id[:])
				if err != nil {
					t.Fatalf("新しい秘密鍵の暗号化のエラー = %v", err)
				}
				newSettingID = testutil.NewUserTwoFactorAuthBuilder(t, db, user.ID).
					WithSecretCiphertext(ciphertext).WithEnabledAt(time.Now()).Build()
				if err := recoveryRepo.CreateAll(ctx, user.ID, []string{"new-code-digest"}); err != nil {
					t.Fatalf("新しいリカバリーコードの作成のエラー = %v", err)
				}
			})

			err := uc.Execute(ctx, usecase.DisableTwoFactorAuthInput{UserID: user.ID, Credential: credential})
			if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
				t.Errorf("設定を作り直したときのエラー = %v、AppErrCodeConflictを期待", err)
			}
			setting, err := repository.NewUserTwoFactorAuthRepository(db).FindByUserID(ctx, user.ID)
			if err != nil || setting == nil || setting.ID != newSettingID || !setting.IsEnabled() || setting.LastUsedStep != 0 {
				t.Errorf("新しい設定 = (%+v, %v)、タイムステップを使わず有効な設定が残ることを期待", setting, err)
			}
			count, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CountUnused(ctx, user.ID)
			if err != nil || count != 1 {
				t.Errorf("新しいリカバリーコードの数 = (%d, %v)、(1, nil) を期待", count, err)
			}
		})
	}
}
