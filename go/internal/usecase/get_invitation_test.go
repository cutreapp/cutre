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

// TestGetInvitationUsecase_Execute は、使える招待を招待した人と一緒に返すことを検証する。
// 招待した人は、管理者が発行した招待と、招待した人が退会した招待ではnilになる。
func TestGetInvitationUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewGetInvitationUsecase(
		repository.NewInvitationRepository(db).WithTx(tx),
		repository.NewInvitationRedemptionRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	withdrawnID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()

	tests := []struct {
		name        string
		builder     *testutil.InvitationBuilder
		wantInviter *model.UserID
	}{
		{name: "ユーザーが発行した招待", builder: testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID), wantInviter: &inviterID},
		{name: "管理者が発行した招待", builder: testutil.NewInvitationBuilder(t, tx)},
		{name: "招待した人が退会した招待", builder: testutil.NewInvitationBuilder(t, tx).WithInviterUserID(withdrawnID)},
	}

	for _, tt := range tests {
		id := tt.builder.Build()

		output, err := uc.Execute(ctx, usecase.GetInvitationInput{Token: tt.builder.Token()})
		if err != nil {
			t.Fatalf("%s: Execute()のエラー = %v", tt.name, err)
		}
		if output.Invitation.ID != id {
			t.Errorf("%s: 招待のID = %v、期待値 = %v", tt.name, output.Invitation.ID, id)
		}
		switch {
		case tt.wantInviter == nil && output.Inviter != nil:
			t.Errorf("%s: 招待した人 = %v、期待値 = nil", tt.name, output.Inviter)
		case tt.wantInviter != nil && (output.Inviter == nil || output.Inviter.ID != *tt.wantInviter):
			t.Errorf("%s: 招待した人 = %v、期待値 = %v", tt.name, output.Inviter, *tt.wantInviter)
		}
	}
}

// TestGetInvitationUsecase_Execute_Unusable は、無い招待と使えない招待をどれもリソース未存在として返すことを検証する。
// 人数の上限は、ユーザーの招待では招待者の取り消した招待の使用も合わせて数える。
func TestGetInvitationUsecase_Execute_Unusable(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	uc := usecase.NewGetInvitationUsecase(
		repository.NewInvitationRepository(db).WithTx(tx),
		repository.NewInvitationRedemptionRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)

	past := time.Now().Add(-time.Minute)

	usedAdmin := testutil.NewInvitationBuilder(t, tx)
	testutil.NewInvitationRedemptionBuilder(t, tx, usedAdmin.Build()).Build()

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	revokedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithRevokedAt(past).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, revokedID).Build()
	full := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID)
	fullID := full.Build()
	for range model.InviterRedemptionLimit - 1 {
		testutil.NewInvitationRedemptionBuilder(t, tx, fullID).Build()
	}

	tokens := map[string]string{
		"存在しない招待":         "no-such-invitation",
		"使用済みの管理者の招待":     usedAdmin.Token(),
		"招待者の人数の上限に達した招待": full.Token(),
	}
	for name, builder := range map[string]*testutil.InvitationBuilder{
		"期限切れの招待":   testutil.NewInvitationBuilder(t, tx).WithExpiresAt(past),
		"取り消し済みの招待": testutil.NewInvitationBuilder(t, tx).WithRevokedAt(past),
	} {
		builder.Build()
		tokens[name] = builder.Token()
	}

	for name, token := range tokens {
		_, err := uc.Execute(ctx, usecase.GetInvitationInput{Token: token})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: Execute()のエラー = %v、AppErrCodeResourceNotFoundのAppErrorを期待", name, err)
		}
	}
}
