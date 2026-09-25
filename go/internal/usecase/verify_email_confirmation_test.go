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

// TestVerifyEmailConfirmationUsecase_Execute は、コードの照合の結果に応じて、確認済みの確認か、
// コードの欄のエラー・再送を促すフォーム全体のエラーを返すことを検証する。
func TestVerifyEmailConfirmationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)
	uc := usecase.NewVerifyEmailConfirmationUsecase(
		validator.NewEmailConfirmationCreateValidator(),
		repository.NewEmailConfirmationRepository(db).WithTx(tx),
	)
	const unusable = "この確認コードは使えなくなりました。コードを再送して、新しいコードを入力してください"

	tests := []struct {
		name          string
		builder       *testutil.EmailConfirmationBuilder
		code          string
		wantConfirmed bool
		wantFieldErr  string
		wantGlobalErr string
	}{
		{name: "一致するコード", builder: testutil.NewEmailConfirmationBuilder(t, tx), code: "123456", wantConfirmed: true},
		{name: "一致しないコード", builder: testutil.NewEmailConfirmationBuilder(t, tx), code: "000000", wantFieldErr: "確認コードが正しくありません"},
		{name: "形式の誤りは照合しない", builder: testutil.NewEmailConfirmationBuilder(t, tx), code: "12345", wantFieldErr: "6桁の数字で入力してください"},
		{
			name:          "上限に達する誤入力",
			builder:       testutil.NewEmailConfirmationBuilder(t, tx).WithFailedAttemptsCount(model.EmailConfirmationMaxFailedAttempts - 1),
			code:          "000000",
			wantGlobalErr: unusable,
		},
		{
			name:          "上限に達したあとの正しいコード",
			builder:       testutil.NewEmailConfirmationBuilder(t, tx).WithFailedAttemptsCount(model.EmailConfirmationMaxFailedAttempts),
			code:          "123456",
			wantGlobalErr: unusable,
		},
		{
			name:          "期限切れ",
			builder:       testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(time.Now().Add(-time.Minute)),
			code:          "123456",
			wantGlobalErr: unusable,
		},
		{
			name:          "確認済み",
			builder:       testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()),
			code:          "123456",
			wantGlobalErr: unusable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := uc.Execute(ctx, usecase.VerifyEmailConfirmationInput{ID: tt.builder.Build(), Code: tt.code})

			if tt.wantConfirmed {
				if err != nil {
					t.Fatalf("Execute()のエラー = %v", err)
				}
				if output.EmailConfirmation.ConfirmedAt == nil {
					t.Errorf("確認 = %+v、確認済みを期待", output.EmailConfirmation)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Execute()のエラー = %v、ValidationErrorを期待", err)
			}
			if tt.wantFieldErr != "" {
				if got := ve.GetFieldErrors("code"); len(got) != 1 || got[0] != tt.wantFieldErr {
					t.Errorf("codeのエラー = %v、期待値 = [%s]", got, tt.wantFieldErr)
				}
			}
			if tt.wantGlobalErr != "" {
				if len(ve.Global) != 1 || ve.Global[0] != tt.wantGlobalErr {
					t.Errorf("フォーム全体のエラー = %v、期待値 = [%s]", ve.Global, tt.wantGlobalErr)
				}
			}
		})
	}
}
