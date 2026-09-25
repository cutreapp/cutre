package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetConfirmedEmailConfirmationUsecase_Execute は、確認済みの確認を返し、未確認の確認には ResourceNotFound を返すことを検証する。
func TestGetConfirmedEmailConfirmationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetConfirmedEmailConfirmationUsecase(repository.NewEmailConfirmationRepository(db).WithTx(tx))

	confirmedID := testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build()
	output, err := uc.Execute(context.Background(), confirmedID)
	if err != nil || output.EmailConfirmation.ID != confirmedID {
		t.Errorf("Execute(確認済み) = (%+v, %v)、確認 %v を期待", output, err, confirmedID)
	}

	_, err = uc.Execute(context.Background(), testutil.NewEmailConfirmationBuilder(t, tx).Build())
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
		t.Errorf("Execute(未確認)のエラー = %v、ResourceNotFoundを期待", err)
	}
}
