package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestInvitationRedemptionRepository_Create は、招待とユーザーを結んだ使用の記録を作ることを検証する。
func TestInvitationRedemptionRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRedemptionRepository(db).WithTx(tx)
	invitationID := testutil.NewInvitationBuilder(t, tx).Build()
	userID := testutil.NewUserBuilder(t, tx).Build()

	redemption, err := repo.Create(ctx, invitationID, userID)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if redemption.InvitationID != invitationID || redemption.UserID != userID {
		t.Errorf("使用の記録 = %+v、招待 %v・ユーザー %v を期待", redemption, invitationID, userID)
	}
}

// TestInvitationRedemptionRepository_Create_SameUser は、1人が複数の招待で登録した記録を作れないことを検証する。
func TestInvitationRedemptionRepository_Create_SameUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRedemptionRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()

	if _, err := repo.Create(ctx, testutil.NewInvitationBuilder(t, tx).Build(), userID); err != nil {
		t.Fatalf("1件目のCreate()のエラー = %v", err)
	}
	if _, err := repo.Create(ctx, testutil.NewInvitationBuilder(t, tx).Build(), userID); err == nil {
		t.Error("同じユーザーの2件目の使用の記録でエラーを期待したが、nilだった")
	}
}

// TestInvitationRedemptionRepository_Count は、招待ごとの件数と、
// 取り消した招待を含む招待者のすべての招待の件数を数えることを検証する。
func TestInvitationRedemptionRepository_Count(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRedemptionRepository(db).WithTx(tx)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	otherInviterID := testutil.NewUserBuilder(t, tx).Build()
	revokedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithRevokedAt(time.Now()).Build()
	currentID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).Build()
	otherID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(otherInviterID).Build()
	unusedID := testutil.NewInvitationBuilder(t, tx).Build()

	testutil.NewInvitationRedemptionBuilder(t, tx, revokedID).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, currentID).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, currentID).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, otherID).Build()

	for name, tt := range map[string]struct {
		count func() (int, error)
		want  int
	}{
		"招待ごと (複数人が使った招待)": {count: func() (int, error) { return repo.CountByInvitationID(ctx, currentID) }, want: 2},
		"招待ごと (未使用の招待)":    {count: func() (int, error) { return repo.CountByInvitationID(ctx, unusedID) }, want: 0},
		"招待者ごと":            {count: func() (int, error) { return repo.CountByInviterUserID(ctx, inviterID) }, want: 3},
		"招待していない招待者":       {count: func() (int, error) { return repo.CountByInviterUserID(ctx, testutil.NewUserBuilder(t, tx).Build()) }, want: 0},
	} {
		got, err := tt.count()
		if err != nil {
			t.Fatalf("%s: 件数の取得のエラー = %v", name, err)
		}
		if got != tt.want {
			t.Errorf("%s: 件数 = %d、期待値 = %d", name, got, tt.want)
		}
	}
}

// TestInvitationRedemptionRepository_ListWithUsersByInviterUserID は、取り消した招待を含む招待者のすべての招待の使用を、
// 登録したユーザーと合わせて新しい順に返し、退会したユーザーも退会した時刻付きで含めることを検証する。
func TestInvitationRedemptionRepository_ListWithUsersByInviterUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRedemptionRepository(db).WithTx(tx)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	revokedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithRevokedAt(time.Now()).Build()
	currentID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).Build()
	otherID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(testutil.NewUserBuilder(t, tx).Build()).Build()

	oldAtname := testutil.UniqueAtname()
	oldUserID := testutil.NewUserBuilder(t, tx).WithAtname(oldAtname).Build()
	deletedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	withdrawnUserID := testutil.NewUserBuilder(t, tx).WithDeletedAt(deletedAt).Build()
	newAtname := testutil.UniqueAtname()
	newUserID := testutil.NewUserBuilder(t, tx).WithAtname(newAtname).Build()

	now := time.Now()
	testutil.NewInvitationRedemptionBuilder(t, tx, revokedID).WithUserID(oldUserID).WithCreatedAt(now.Add(-3 * time.Hour)).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, currentID).WithUserID(withdrawnUserID).WithCreatedAt(now.Add(-2 * time.Hour)).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, currentID).WithUserID(newUserID).WithCreatedAt(now.Add(-time.Hour)).Build()
	testutil.NewInvitationRedemptionBuilder(t, tx, otherID).Build()

	redemptions, err := repo.ListWithUsersByInviterUserID(ctx, inviterID)
	if err != nil {
		t.Fatalf("ListWithUsersByInviterUserID()のエラー = %v", err)
	}
	if len(redemptions) != 3 {
		t.Fatalf("件数 = %d、期待値 = 3", len(redemptions))
	}

	wantUserIDs := []model.UserID{newUserID, withdrawnUserID, oldUserID}
	for i, redemption := range redemptions {
		if redemption.UserID != wantUserIDs[i] || redemption.User == nil || redemption.User.ID != wantUserIDs[i] {
			t.Errorf("%d件目 = %+v、ユーザー %v を期待", i, redemption, wantUserIDs[i])
		}
	}
	if got := redemptions[0].User; got.Atname != newAtname || got.DeletedAt != nil {
		t.Errorf("在籍中のユーザー = %+v、アットネーム %q・退会していないことを期待", got, newAtname)
	}
	if got := redemptions[1].User.DeletedAt; got == nil || !got.Equal(deletedAt) {
		t.Errorf("退会したユーザーの DeletedAt = %v、期待値 = %v", got, deletedAt)
	}
	if got := redemptions[2].User.Atname; got != oldAtname {
		t.Errorf("取り消した招待で登録したユーザーのアットネーム = %q、期待値 = %q", got, oldAtname)
	}
}

// TestInvitationRedemptionRepository_ListWithUsersByInviterUserID_Empty は、招待で誰も登録していなければ空の一覧を返すことを検証する。
func TestInvitationRedemptionRepository_ListWithUsersByInviterUserID_Empty(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewInvitationRedemptionRepository(db).WithTx(tx)

	redemptions, err := repo.ListWithUsersByInviterUserID(context.Background(), testutil.NewUserBuilder(t, tx).Build())
	if err != nil {
		t.Fatalf("ListWithUsersByInviterUserID()のエラー = %v", err)
	}
	if len(redemptions) != 0 {
		t.Errorf("件数 = %d、期待値 = 0", len(redemptions))
	}
}
