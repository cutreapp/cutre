package usecase_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetTwoFactorAuthStatusUsecase_Execute は、有効にした設定と未使用のリカバリーコードの数を返し、
// 設定が無いときと登録の途中のときは無効として返すことを検証する。
func TestGetTwoFactorAuthStatusUsecase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T, tx *sql.Tx, recoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository, userID model.UserID)
		wantEnabled bool
		wantCount   int
	}{
		{name: "設定が無い", setup: func(*testing.T, *sql.Tx, *repository.UserTwoFactorRecoveryCodeRepository, model.UserID) {}},
		{name: "登録の途中", setup: func(t *testing.T, tx *sql.Tx, _ *repository.UserTwoFactorRecoveryCodeRepository, userID model.UserID) {
			testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).Build()
		}},
		{name: "有効", wantEnabled: true, wantCount: 2, setup: func(t *testing.T, tx *sql.Tx, recoveryCodeRepo *repository.UserTwoFactorRecoveryCodeRepository, userID model.UserID) {
			testutil.NewUserTwoFactorAuthBuilder(t, tx, userID).WithEnabledAt(time.Now()).Build()
			if err := recoveryCodeRepo.CreateAll(t.Context(), userID, []string{"digest-1", "digest-2", "digest-3"}); err != nil {
				t.Fatalf("リカバリーコードの作成のエラー = %v", err)
			}
			if _, err := recoveryCodeRepo.Use(t.Context(), userID, "digest-1"); err != nil {
				t.Fatalf("リカバリーコードの使用のエラー = %v", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			recoveryCodeRepo := repository.NewUserTwoFactorRecoveryCodeRepository(db).WithTx(tx)
			userID := testutil.NewUserBuilder(t, tx).Build()
			tt.setup(t, tx, recoveryCodeRepo, userID)

			uc := usecase.NewGetTwoFactorAuthStatusUsecase(repository.NewUserTwoFactorAuthRepository(db).WithTx(tx), recoveryCodeRepo)
			output, err := uc.Execute(t.Context(), usecase.GetTwoFactorAuthStatusInput{UserID: userID})
			if err != nil {
				t.Fatalf("Execute()のエラー = %v", err)
			}
			if got := output.TwoFactorAuth != nil; got != tt.wantEnabled {
				t.Errorf("有効な設定の有無 = %v、期待値 = %v", got, tt.wantEnabled)
			}
			if output.UnusedRecoveryCodeCount != tt.wantCount {
				t.Errorf("未使用のリカバリーコードの数 = %d、期待値 = %d", output.UnusedRecoveryCodeCount, tt.wantCount)
			}
		})
	}
}
