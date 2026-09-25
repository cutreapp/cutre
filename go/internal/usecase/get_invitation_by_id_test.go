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

// TestGetInvitationByIDUsecase_Execute は、使える招待だけを返し、無い・使えない招待はリソース未存在にすることを検証する。
func TestGetInvitationByIDUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewGetInvitationByIDUsecase(
		repository.NewInvitationRepository(db).WithTx(tx),
		repository.NewInvitationRedemptionRepository(db).WithTx(tx),
	)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	partlyUsedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, partlyUsedID).Build()

	for name, id := range map[string]model.InvitationID{
		"未使用の招待":          testutil.NewInvitationBuilder(t, tx).Build(),
		"人数に空きのあるユーザーの招待": partlyUsedID,
	} {
		output, err := uc.Execute(ctx, id)
		if err != nil || output.Invitation.ID != id {
			t.Errorf("%s: Execute() = (%+v, %v)、招待 %v を期待", name, output, err, id)
		}
	}

	usedAdminID := testutil.NewInvitationBuilder(t, tx).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, usedAdminID).Build()

	for name, id := range map[string]model.InvitationID{
		"存在しない招待":     model.InvitationID(uuid.New()),
		"取り消し済みの招待":   testutil.NewInvitationBuilder(t, tx).WithRevokedAt(time.Now()).Build(),
		"使用済みの管理者の招待": usedAdminID,
	} {
		_, err := uc.Execute(ctx, id)
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFoundのAppErrorを期待", name, err)
		}
	}
}
