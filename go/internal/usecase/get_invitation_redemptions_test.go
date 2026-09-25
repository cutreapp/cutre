package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetInvitationRedemptionsUsecase_Execute は、招待者の招待で登録した人を、退会した人も含めて新しい順に返すことを検証する。
func TestGetInvitationRedemptionsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetInvitationRedemptionsUsecase(repository.NewInvitationRedemptionRepository(db).WithTx(tx))

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	invitationID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).Build()
	withdrawnID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()
	memberID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).WithUserID(withdrawnID).WithCreatedAt(time.Now().Add(-time.Hour)).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, invitationID).WithUserID(memberID).Build()

	output, err := uc.Execute(context.Background(), usecase.GetInvitationRedemptionsInput{InviterUserID: inviterID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	redemptions := output.Redemptions
	if len(redemptions) != 2 {
		t.Fatalf("件数 = %d、期待値 = 2", len(redemptions))
	}
	if redemptions[0].User.ID != memberID || redemptions[0].User.DeletedAt != nil {
		t.Errorf("1件目の登録した人 = %+v、在籍中のユーザー %v を期待", redemptions[0].User, memberID)
	}
	if redemptions[1].User.ID != withdrawnID || redemptions[1].User.DeletedAt == nil {
		t.Errorf("2件目の登録した人 = %+v、退会したユーザー %v を期待", redemptions[1].User, withdrawnID)
	}
}
