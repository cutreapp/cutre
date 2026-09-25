package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/i18n"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestCreateSignInUsecase_Execute は、資格情報が一致すればユーザーを返し、
// 一致しなければバリデーターのエラーをそのまま返すことを検証する。
// 一致しない理由ごとの区別はバリデーターのテストが確かめる。
func TestCreateSignInUsecase_Execute(t *testing.T) {
	t.Parallel()

	const password = "password123"

	tests := []struct {
		name string
		// twoFactorAuth はユーザーの二要素認証の設定。"" は設定無し、"pending" は登録の途中、"enabled" は有効。
		twoFactorAuth             string
		password                  string
		wantUser                  bool
		wantTwoFactorAuthRequired bool
	}{
		{name: "資格情報が一致すればユーザーを返す", password: password, wantUser: true},
		{name: "一致しなければValidationErrorを返す", password: "password124"},
		{name: "二要素認証を有効にしていればコードを求める", twoFactorAuth: "enabled", password: password, wantUser: true, wantTwoFactorAuthRequired: true},
		{name: "認証アプリへの登録の途中ならコードを求めない", twoFactorAuth: "pending", password: password, wantUser: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
			uc := usecase.NewCreateSignInUsecase(
				validator.NewSignInCreateValidator(
					repository.NewUserRepository(db).WithTx(tx),
					repository.NewUserPasswordRepository(db).WithTx(tx),
				),
				repository.NewUserTwoFactorAuthRepository(db).WithTx(tx),
			)

			email := testutil.UniqueEmail("sign-in")
			userID := testutil.NewUserBuilder(t, tx).WithEmail(email).Build()
			testutil.NewUserPasswordBuilder(t, tx).WithUserID(userID).WithPassword(password).Build()
			switch tt.twoFactorAuth {
			case "pending":
				testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).Build()
			case "enabled":
				testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).WithEnabledAt(time.Now()).Build()
			}

			output, err := uc.Execute(ctx, usecase.CreateSignInInput{Email: email, Password: tt.password})

			if !tt.wantUser {
				if model.AsValidationError(err) == nil {
					t.Errorf("エラー = %v、ValidationErrorを期待", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute()のエラー = %v", err)
			}
			if output.User.ID != userID {
				t.Errorf("UserID = %s、期待値 = %s", output.User.ID, userID)
			}
			if output.TwoFactorAuthRequired != tt.wantTwoFactorAuthRequired {
				t.Errorf("TwoFactorAuthRequired = %t、期待値 = %t", output.TwoFactorAuthRequired, tt.wantTwoFactorAuthRequired)
			}
		})
	}
}
