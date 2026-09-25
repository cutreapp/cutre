package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newTestTwoFactorKey はテスト用の鍵の TwoFactorKey を返す。
func newTestTwoFactorKey(t *testing.T) *auth.TwoFactorKey {
	t.Helper()

	key, err := auth.NewTwoFactorKey("test-totp-encryption-key-0123456789")
	if err != nil {
		t.Fatalf("鍵の作成のエラー = %v", err)
	}

	return key
}

// newEnableTwoFactorAuthUsecase はテスト用のデータベースに直接書き込む EnableTwoFactorAuthUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newEnableTwoFactorAuthUsecase(key *auth.TwoFactorKey) *usecase.EnableTwoFactorAuthUsecase {
	db := testutil.GetTestDB()
	return usecase.NewEnableTwoFactorAuthUsecase(
		db,
		key,
		validator.NewTwoFactorAuthCreateValidator(),
		repository.NewUserTwoFactorAuthRepository(db),
		repository.NewUserTwoFactorRecoveryCodeRepository(db),
	)
}

// twoFactorUser はテスト用のユーザーを作って返す。
func twoFactorUser(t *testing.T) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("ユーザーの取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// prepareTwoFactorAuth はユーザーの認証アプリへの登録の途中の設定を作り、その秘密鍵を返す。
func prepareTwoFactorAuth(t *testing.T, key *auth.TwoFactorKey, user *model.User) string {
	t.Helper()

	uc := usecase.NewPrepareTwoFactorAuthUsecase(key, repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()))
	output, err := uc.Execute(context.Background(), usecase.PrepareTwoFactorAuthInput{User: user})
	if err != nil || output.Setup == nil {
		t.Fatalf("登録の準備 = (%+v, %v)、秘密鍵を期待", output, err)
	}

	return output.Setup.Secret
}

// currentTOTPCode は秘密鍵の今のコードを返す。
func currentTOTPCode(t *testing.T, secret string) string {
	t.Helper()

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コードの生成のエラー = %v", err)
	}

	return code
}

// TestEnableTwoFactorAuthUsecase_Execute は、コードを照合して二要素認証を有効にし、
// 照合できるダイジェストで保存したリカバリーコードを10件返すことを検証する。
// 貼り付けで混じった空白と全角の数字は取り除いてから照合する。
func TestEnableTwoFactorAuthUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	code := currentTOTPCode(t, prepareTwoFactorAuth(t, key, user))
	input := code[:3] + " " + string([]rune("０１２３４５６７８９")[code[3]-'0']) + code[4:]

	output, err := newEnableTwoFactorAuthUsecase(key).Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: input})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.RecoveryCodes) != auth.RecoveryCodeCount {
		t.Fatalf("リカバリーコードの数 = %d、期待値 = %d", len(output.RecoveryCodes), auth.RecoveryCodeCount)
	}

	twoFactorAuth, err := repository.NewUserTwoFactorAuthRepository(db).FindByUserID(ctx, user.ID)
	if err != nil || twoFactorAuth == nil || !twoFactorAuth.IsEnabled() || twoFactorAuth.LastUsedStep == 0 {
		t.Errorf("二要素認証の設定 = (%+v, %v)、使ったステップを記録した有効な設定を期待", twoFactorAuth, err)
	}
	recoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db)
	if count, err := recoveryCodeRepo.CountUnused(ctx, user.ID); err != nil || count != auth.RecoveryCodeCount {
		t.Errorf("未使用のリカバリーコードの数 = (%d, %v)、期待値 = %d", count, err, auth.RecoveryCodeCount)
	}
	digest := key.RecoveryCodeDigest(auth.NormalizeRecoveryCode(output.RecoveryCodes[0]))
	if used, err := recoveryCodeRepo.Use(ctx, user.ID, digest); err != nil || !used {
		t.Errorf("返したリカバリーコードの使用 = (%v, %v)、使えることを期待", used, err)
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_InvalidCode は、形式の誤りと一致しないコードをコードの欄のエラーで返し、
// 有効にしないことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_InvalidCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code func(secret string) string
	}{
		{name: "空", code: func(string) string { return "" }},
		{name: "5桁", code: func(string) string { return "12345" }},
		{name: "数字以外", code: func(string) string { return "12345a" }},
		{name: "一致しない", code: func(secret string) string {
			code := currentTOTPCode(t, secret)
			return string('0'+(code[0]-'0'+1)%10) + code[1:]
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			key := newTestTwoFactorKey(t)
			user := twoFactorUser(t)
			secret := prepareTwoFactorAuth(t, key, user)

			_, err := newEnableTwoFactorAuthUsecase(key).Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: tt.code(secret)})

			if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("code") {
				t.Fatalf("Execute()のエラー = %v、コードの欄の ValidationError を期待", err)
			}
			twoFactorAuth, err := repository.NewUserTwoFactorAuthRepository(testutil.GetTestDB()).FindByUserID(ctx, user.ID)
			if err != nil || twoFactorAuth == nil || twoFactorAuth.IsEnabled() {
				t.Errorf("二要素認証の設定 = (%+v, %v)、登録の途中のままを期待", twoFactorAuth, err)
			}
		})
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_NotPending は、登録の途中の設定が無い (登録の画面を開いていない・既に有効にした) とき、
// AppErrCodeConflict を返し、リカバリーコードを作らないことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_NotPending(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(t *testing.T, userID model.UserID)
	}{
		{name: "登録の画面を開いていない", prepare: func(*testing.T, model.UserID) {}},
		{name: "既に有効にした", prepare: func(t *testing.T, userID model.UserID) {
			testutil.NewUserTwoFactorAuthBuilder(t, testutil.GetTestDB(), userID).WithEnabledAt(time.Now()).Build()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			user := twoFactorUser(t)
			tt.prepare(t, user.ID)

			_, err := newEnableTwoFactorAuthUsecase(newTestTwoFactorKey(t)).Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: "123456"})

			if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
				t.Fatalf("Execute()のエラー = %v、AppErrCodeConflict を期待", err)
			}
			count, err := repository.NewUserTwoFactorRecoveryCodeRepository(testutil.GetTestDB()).CountUnused(ctx, user.ID)
			if err != nil || count != 0 {
				t.Errorf("リカバリーコードの数 = (%d, %v)、期待値 = 0", count, err)
			}
		})
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_Duplicate は、同じコードで二度送られても有効にするのは1回だけで、
// 2回目は AppErrCodeConflict を返してリカバリーコードを作り直さないことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_Duplicate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	code := currentTOTPCode(t, prepareTwoFactorAuth(t, key, user))
	uc := newEnableTwoFactorAuthUsecase(key)

	first, err := uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: code})
	if err != nil {
		t.Fatalf("1回目のExecute()のエラー = %v", err)
	}
	_, err = uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: code})
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Fatalf("2回目のExecute()のエラー = %v、AppErrCodeConflict を期待", err)
	}

	digest := key.RecoveryCodeDigest(auth.NormalizeRecoveryCode(first.RecoveryCodes[0]))
	if used, err := repository.NewUserTwoFactorRecoveryCodeRepository(testutil.GetTestDB()).Use(ctx, user.ID, digest); err != nil || !used {
		t.Errorf("1回目のリカバリーコードの使用 = (%v, %v)、使えることを期待", used, err)
	}
}

// TestEnableTwoFactorAuthUsecase_Execute_SecretReplacedAfterMatch は、コードを照合した後、有効にする前に
// 別の画面で秘密鍵が作り直されたとき、AppErrCodeConflict を返し、新しい秘密鍵の設定を有効にせず、
// リカバリーコードも作らないことを検証する。
func TestEnableTwoFactorAuthUsecase_Execute_SecretReplacedAfterMatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	key := newTestTwoFactorKey(t)
	user := twoFactorUser(t)
	code := currentTOTPCode(t, prepareTwoFactorAuth(t, key, user))
	uc := newEnableTwoFactorAuthUsecase(key)
	uc.SetBeforeEnableHook(func(context.Context) {
		prepareTwoFactorAuth(t, key, user)
	})

	_, err := uc.Execute(ctx, usecase.EnableTwoFactorAuthInput{UserID: user.ID, Code: code})

	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeConflict {
		t.Fatalf("Execute()のエラー = %v、AppErrCodeConflict を期待", err)
	}
	db := testutil.GetTestDB()
	if twoFactorAuth, err := repository.NewUserTwoFactorAuthRepository(db).FindByUserID(ctx, user.ID); err != nil || twoFactorAuth == nil || twoFactorAuth.IsEnabled() {
		t.Errorf("二要素認証の設定 = (%+v, %v)、登録の途中のままを期待", twoFactorAuth, err)
	}
	if count, err := repository.NewUserTwoFactorRecoveryCodeRepository(db).CountUnused(ctx, user.ID); err != nil || count != 0 {
		t.Errorf("リカバリーコードの数 = (%d, %v)、期待値 = 0", count, err)
	}
}
