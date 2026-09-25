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

// TestGetPasswordResetTokenByIDUsecase_Execute は、期限内のトークンとその持ち主を引き、
// 期限切れ・無いトークンと、持ち主が退会したトークンでは AppErrCodeResourceNotFound を返すことを検証する。
func TestGetPasswordResetTokenByIDUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetPasswordResetTokenByIDUsecase(
		repository.NewPasswordResetTokenRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()
	liveID := testutil.NewPasswordResetTokenBuilder(t, tx).WithUserID(userID).Build()
	output, err := uc.Execute(ctx, liveID)
	if err != nil || output.PasswordResetToken.ID != liveID || output.User.ID != userID {
		t.Errorf("期限内のトークンのExecute() = (%+v, %v)、トークン %v とユーザー %v を期待", output, err, liveID, userID)
	}

	tests := []struct {
		name string
		id   model.PasswordResetTokenID
	}{
		{name: "期限切れのトークン", id: testutil.NewPasswordResetTokenBuilder(t, tx).
			WithUserID(testutil.NewUserBuilder(t, tx).Build()).
			WithExpiresAt(time.Now().Add(-time.Minute)).
			Build()},
		{name: "無いトークン", id: model.PasswordResetTokenID(uuid.New())},
		{name: "退会したユーザーのトークン", id: testutil.NewPasswordResetTokenBuilder(t, tx).
			WithUserID(testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()).
			Build()},
	}
	for _, tt := range tests {
		_, err := uc.Execute(ctx, tt.id)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFoundを期待", tt.name, err)
		}
	}
}
