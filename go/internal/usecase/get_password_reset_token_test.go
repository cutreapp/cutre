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

// TestGetPasswordResetTokenUsecase_Execute は、リンクの平文のトークンから期限内のトークンを引き、
// 期限切れと無いトークンでは AppErrCodeResourceNotFound を返すことを検証する。
func TestGetPasswordResetTokenUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetPasswordResetTokenUsecase(repository.NewPasswordResetTokenRepository(db).WithTx(tx))
	ctx := context.Background()

	live := testutil.NewPasswordResetTokenBuilder(t, tx).WithUserID(testutil.NewUserBuilder(t, tx).Build())
	liveID := live.Build()
	output, err := uc.Execute(ctx, usecase.GetPasswordResetTokenInput{Token: live.Token()})
	if err != nil || output.PasswordResetToken.ID != liveID {
		t.Errorf("期限内のトークンのExecute() = (%+v, %v)、ID %v を期待", output, err, liveID)
	}

	expired := testutil.NewPasswordResetTokenBuilder(t, tx).
		WithUserID(testutil.NewUserBuilder(t, tx).Build()).
		WithExpiresAt(time.Now().Add(-time.Minute))
	expired.Build()
	for _, token := range []string{expired.Token(), "no-such-token"} {
		_, err := uc.Execute(ctx, usecase.GetPasswordResetTokenInput{Token: token})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("トークン %q のエラー = %v、AppErrCodeResourceNotFoundを期待", token, err)
		}
	}
}
