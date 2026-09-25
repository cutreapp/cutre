package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/auth"
	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestInvitationRepository_Create は、招待者の有無のどちらでも招待を作り、未取り消しの状態で返すことを検証する。
func TestInvitationRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	inviterID := testutil.NewUserBuilder(t, tx).Build()

	tests := []struct {
		name      string
		inviterID *model.UserID
	}{
		{name: "管理者が発行した招待 (招待者無し)", inviterID: nil},
		{name: "ユーザーが発行した招待", inviterID: &inviterID},
	}

	for _, tt := range tests {
		token, err := auth.GenerateSecureToken()
		if err != nil {
			t.Fatalf("GenerateSecureToken()のエラー = %v", err)
		}
		expiresAt := time.Now().Add(model.InvitationLifetime).Truncate(time.Microsecond)

		invitation, err := repo.Create(ctx, repository.CreateInvitationInput{
			InviterUserID: tt.inviterID,
			Token:         token,
			ExpiresAt:     expiresAt,
		})
		if err != nil {
			t.Fatalf("%s: Create()のエラー = %v", tt.name, err)
		}

		if (invitation.InviterUserID == nil) != (tt.inviterID == nil) ||
			(tt.inviterID != nil && *invitation.InviterUserID != *tt.inviterID) {
			t.Errorf("%s: InviterUserID = %v、期待値 = %v", tt.name, invitation.InviterUserID, tt.inviterID)
		}
		if invitation.Token != token {
			t.Errorf("%s: Token = %q、期待値 = %q", tt.name, invitation.Token, token)
		}
		if !invitation.ExpiresAt.Equal(expiresAt) {
			t.Errorf("%s: ExpiresAt = %v、期待値 = %v", tt.name, invitation.ExpiresAt, expiresAt)
		}
		if invitation.RevokedAt != nil {
			t.Errorf("%s: RevokedAt = %v、nilを期待", tt.name, invitation.RevokedAt)
		}
	}
}

// TestInvitationRepository_Create_DuplicateToken は、同じトークンの招待を2つ作れないことを検証する。
// トークンは招待リンクから招待を1つに引くための値のため、重複すると別の招待を使わせてしまう。
func TestInvitationRepository_Create_DuplicateToken(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	input := repository.CreateInvitationInput{Token: "duplicate-" + testutil.UniqueAtname(), ExpiresAt: time.Now().Add(time.Hour)}
	if _, err := repo.Create(ctx, input); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repo.Create(ctx, input); err == nil {
		t.Error("同じトークンの2つ目の招待でエラーを期待したが、nilだった")
	}
}

// TestInvitation_UnrevokedPerInviter は、取り消していない招待を招待者ごとに1本しか持てないことを検証する。
// 取り消した招待と、招待者の無い (管理者が発行した) 招待は数に入らない。
func TestInvitation_UnrevokedPerInviter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		withInviter bool
		revokeFirst bool
		wantErr     bool
	}{
		{name: "同じ招待者の取り消していない招待が2本", withInviter: true, wantErr: true},
		{name: "取り消した招待と取り消していない招待", withInviter: true, revokeFirst: true},
		{name: "管理者が発行した招待が2本"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, tx := testutil.SetupTx(t)
			ctx := context.Background()
			repo := repository.NewInvitationRepository(db).WithTx(tx)

			var inviterID *model.UserID
			if tt.withInviter {
				id := testutil.NewUserBuilder(t, tx).Build()
				inviterID = &id
			}
			first := testutil.NewInvitationBuilder(t, tx)
			if inviterID != nil {
				first = first.WithInviterUserID(*inviterID)
			}
			if tt.revokeFirst {
				first = first.WithRevokedAt(time.Now())
			}
			first.Build()

			_, err := repo.Create(ctx, repository.CreateInvitationInput{
				InviterUserID: inviterID,
				Token:         "unrevoked-" + testutil.UniqueAtname(),
				ExpiresAt:     time.Now().Add(model.InvitationLifetime),
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("2本目のCreate()のエラー = %v、エラー期待 = %t", err, tt.wantErr)
			}
		})
	}
}

// TestInvitationRepository_FindByToken は、トークンで招待を引き、使えない招待も区別せずに返し、
// 無いトークンでは (nil, nil) を返すことを検証する。
func TestInvitationRepository_FindByToken(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	revoked := testutil.NewInvitationBuilder(t, tx).WithRevokedAt(time.Now())
	id := revoked.Build()

	found, err := repo.FindByToken(ctx, revoked.Token())
	if err != nil {
		t.Fatalf("FindByToken()のエラー = %v", err)
	}
	if found == nil || found.ID != id || found.RevokedAt == nil {
		t.Errorf("FindByToken() = %+v、取り消し済みの招待 %v を期待", found, id)
	}

	missing, err := repo.FindByToken(ctx, "no-such-invitation")
	if err != nil || missing != nil {
		t.Errorf("FindByToken(無いトークン) = (%v, %v)、(nil, nil)を期待", missing, err)
	}
}

// TestInvitationRepository_FindByID は、IDで招待を引き、無いIDでは (nil, nil) を返すことを検証する。
func TestInvitationRepository_FindByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	id := testutil.NewInvitationBuilder(t, tx).Build()

	found, err := repo.FindByID(ctx, id)
	if err != nil || found == nil || found.ID != id {
		t.Errorf("FindByID() = (%+v, %v)、招待 %v を期待", found, err, id)
	}

	missing, err := repo.FindByID(ctx, model.InvitationID(uuid.New()))
	if err != nil || missing != nil {
		t.Errorf("FindByID(無いID) = (%v, %v)、(nil, nil)を期待", missing, err)
	}
}

// TestInvitationRepository_FindByIDWithLock は、ロック付きでIDの招待を引き、無いIDでは (nil, nil) を返すことを検証する。
func TestInvitationRepository_FindByIDWithLock(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	id := testutil.NewInvitationBuilder(t, tx).Build()

	finders := map[string]func(context.Context, model.InvitationID) (*model.Invitation, error){
		"FindByIDForShare":  repo.FindByIDForShare,
		"FindByIDForUpdate": repo.FindByIDForUpdate,
	}
	for name, find := range finders {
		found, err := find(ctx, id)
		if err != nil || found == nil || found.ID != id {
			t.Errorf("%s() = (%+v, %v)、招待 %v を期待", name, found, err, id)
		}

		missing, err := find(ctx, model.InvitationID(uuid.New()))
		if err != nil || missing != nil {
			t.Errorf("%s(無いID) = (%v, %v)、(nil, nil)を期待", name, missing, err)
		}
	}
}

// TestInvitationRepository_FindUnrevokedByInviterUserID は、招待者の取り消していない招待を期限切れでも返し、
// 取り消した招待と他の招待者の招待は返さないことを検証する。
func TestInvitationRepository_FindUnrevokedByInviterUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithRevokedAt(time.Now()).Build()
	expiredID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithExpiresAt(time.Now().Add(-time.Hour)).Build()
	testutil.NewInvitationBuilder(t, tx).WithInviterUserID(testutil.NewUserBuilder(t, tx).Build()).Build()

	invitation, err := repo.FindUnrevokedByInviterUserID(ctx, inviterID)
	if err != nil {
		t.Fatalf("FindUnrevokedByInviterUserID()のエラー = %v", err)
	}
	if invitation == nil || invitation.ID != expiredID {
		t.Errorf("招待 = %+v、期限切れの招待 %v を期待", invitation, expiredID)
	}

	invitation, err = repo.FindUnrevokedByInviterUserID(ctx, testutil.NewUserBuilder(t, tx).Build())
	if err != nil || invitation != nil {
		t.Errorf("招待していない招待者の招待 = (%+v, %v)、(nil, nil) を期待", invitation, err)
	}
}

// TestInvitationRepository_CreateUnlessUnrevokedExists は、招待者が取り消していない招待を持たなければ作り、
// 持っていれば作らずに (nil, nil) を返すことを検証する。
func TestInvitationRepository_CreateUnlessUnrevokedExists(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)
	inviterID := testutil.NewUserBuilder(t, tx).Build()
	expiresAt := time.Now().Add(model.InvitationLifetime).Truncate(time.Microsecond)
	input := func() repository.CreateInvitationInput {
		return repository.CreateInvitationInput{InviterUserID: &inviterID, Token: "unless-" + testutil.UniqueAtname(), ExpiresAt: expiresAt}
	}

	first := input()
	created, err := repo.CreateUnlessUnrevokedExists(ctx, first)
	if err != nil {
		t.Fatalf("1本目のCreateUnlessUnrevokedExists()のエラー = %v", err)
	}
	if created == nil || created.Token != first.Token || created.InviterUserID == nil || *created.InviterUserID != inviterID || !created.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("1本目の招待 = %+v、トークン %q・招待者 %v・期限 %v を期待", created, first.Token, inviterID, expiresAt)
	}

	second, err := repo.CreateUnlessUnrevokedExists(ctx, input())
	if err != nil || second != nil {
		t.Errorf("2本目のCreateUnlessUnrevokedExists() = (%+v, %v)、(nil, nil) を期待", second, err)
	}

	if err := repo.Revoke(ctx, created.ID); err != nil {
		t.Fatalf("Revoke()のエラー = %v", err)
	}
	third, err := repo.CreateUnlessUnrevokedExists(ctx, input())
	if err != nil || third == nil {
		t.Errorf("取り消した後のCreateUnlessUnrevokedExists() = (%+v, %v)、新しい招待を期待", third, err)
	}
}

// TestInvitationRepository_Revoke は、招待を取り消し、取り消し済みの招待では取り消した時刻を変えないことを検証する。
func TestInvitationRepository_Revoke(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	unrevokedID := testutil.NewInvitationBuilder(t, tx).Build()
	revokedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	revokedID := testutil.NewInvitationBuilder(t, tx).WithRevokedAt(revokedAt).Build()

	for _, id := range []model.InvitationID{unrevokedID, revokedID} {
		if err := repo.Revoke(ctx, id); err != nil {
			t.Fatalf("Revoke()のエラー = %v", err)
		}
	}

	unrevoked, err := repo.FindByID(ctx, unrevokedID)
	if err != nil || unrevoked == nil || unrevoked.RevokedAt == nil {
		t.Errorf("取り消した招待 = (%+v, %v)、取り消した時刻を期待", unrevoked, err)
	}
	revoked, err := repo.FindByID(ctx, revokedID)
	if err != nil || revoked == nil || revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(revokedAt) {
		t.Errorf("取り消し済みの招待 = (%+v, %v)、取り消した時刻 %v のままを期待", revoked, err, revokedAt)
	}
}

// TestInvitationRepository_RevokeUnrevokedByInviterUserID は、招待者の取り消していない招待を取り消し、
// 取り消し済みの招待の時刻とほかの招待者の招待を変えないことを検証する。
func TestInvitationRepository_RevokeUnrevokedByInviterUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewInvitationRepository(db).WithTx(tx)

	inviterID := testutil.NewUserBuilder(t, tx).Build()
	unrevokedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).Build()
	revokedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	revokedID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(inviterID).WithRevokedAt(revokedAt).Build()
	otherID := testutil.NewInvitationBuilder(t, tx).WithInviterUserID(testutil.NewUserBuilder(t, tx).Build()).Build()

	if err := repo.RevokeUnrevokedByInviterUserID(ctx, inviterID); err != nil {
		t.Fatalf("RevokeUnrevokedByInviterUserID()のエラー = %v", err)
	}

	unrevoked, err := repo.FindByID(ctx, unrevokedID)
	if err != nil || unrevoked == nil || unrevoked.RevokedAt == nil {
		t.Errorf("取り消していなかった招待 = (%+v, %v)、取り消した時刻を期待", unrevoked, err)
	}
	revoked, err := repo.FindByID(ctx, revokedID)
	if err != nil || revoked == nil || revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(revokedAt) {
		t.Errorf("取り消し済みの招待 = (%+v, %v)、取り消した時刻 %v のままを期待", revoked, err, revokedAt)
	}
	other, err := repo.FindByID(ctx, otherID)
	if err != nil || other == nil || other.RevokedAt != nil {
		t.Errorf("ほかの招待者の招待 = (%+v, %v)、取り消していないままを期待", other, err)
	}
}
