package usecase_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestCheckHealthUsecase_Execute は、データベースに届くときはhealthyを返すことを検証する。
func TestCheckHealthUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewCheckHealthUsecase(repository.NewHealthRepository(db).WithTx(tx))

	result := uc.Execute(context.Background())

	if !result.DBHealthy {
		t.Errorf("DBHealthy = false、期待値 = true (エラー = %v)", result.DBError)
	}
	if result.DBError != nil {
		t.Errorf("DBError = %v、期待値 = nil", result.DBError)
	}
}

// TestCheckHealthUsecase_Execute_Unreachable は、データベースに届かないときに、
// エラーを返さずunhealthyと原因を返すことを検証する。
func TestCheckHealthUsecase_Execute_Unreachable(t *testing.T) {
	t.Parallel()

	uc := usecase.NewCheckHealthUsecase(repository.NewHealthRepository(testutil.OpenUnreachableDB(t)))

	result := uc.Execute(context.Background())

	if result.DBHealthy {
		t.Error("DBHealthy = true、期待値 = false")
	}
	if result.DBError == nil {
		t.Error("DBError = nil、接続の失敗を期待")
	}
}
