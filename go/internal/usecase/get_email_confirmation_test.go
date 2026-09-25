package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetEmailConfirmationUsecase_Execute は、確認済みでない確認を返し、無い・確認済みのときは
// AppErrCodeResourceNotFound を返すことを検証する。
func TestGetEmailConfirmationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetEmailConfirmationUsecase(repository.NewEmailConfirmationRepository(db).WithTx(tx))

	email := testutil.UniqueEmail("get-email-confirmation")
	output, err := uc.Execute(context.Background(), testutil.NewEmailConfirmationBuilder(t, tx).WithEmail(email).Build())
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.EmailConfirmation.Email != email {
		t.Errorf("メールアドレス = %q、期待値 = %q", output.EmailConfirmation.Email, email)
	}

	for name, id := range map[string]model.EmailConfirmationID{
		"確認済み":  testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build(),
		"存在しない": model.EmailConfirmationID(uuid.New()),
	} {
		_, err := uc.Execute(context.Background(), id)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFoundを期待", name, err)
		}
	}
}
