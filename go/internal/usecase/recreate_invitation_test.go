package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newRecreateInvitationUsecase はテスト用のデータベースに直接書き込む RecreateInvitationUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newRecreateInvitationUsecase() *usecase.RecreateInvitationUsecase {
	db := testutil.GetTestDB()
	return usecase.NewRecreateInvitationUsecase(db, repository.NewUserRepository(db), repository.NewInvitationRepository(db), repository.NewInvitationRedemptionRepository(db))
}

// TestRecreateInvitationUsecase_Execute は、使える招待を取り消し、招待者のタイムゾーンの日付で期限を決めた新しい招待を作ることを検証する。
// 今までの招待での使用は、招待者の人数に数えたまま残る。
func TestRecreateInvitationUsecase_Execute(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, "America/New_York")
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, currentID).Build()

	output, err := newRecreateInvitationUsecase().Execute(ctx, usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: currentID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if !output.Recreated {
		t.Error("作り直したことを示していない")
	}
	invitation := output.Invitation
	if invitation == nil || invitation.ID == currentID || invitation.InviterUserID == nil || *invitation.InviterUserID != user.ID {
		t.Fatalf("招待 = %+v、招待者 %v の新しい招待を期待", invitation, user.ID)
	}
	loc, _ := time.LoadLocation("America/New_York")
	if want := model.UserInvitationExpiresAt(invitation.CreatedAt, loc); !invitation.ExpiresAt.Equal(want) {
		t.Errorf("有効期限 = %v、期待値 = %v", invitation.ExpiresAt, want)
	}

	previous, err := repository.NewInvitationRepository(db).FindByID(ctx, currentID)
	if err != nil || previous == nil || previous.RevokedAt == nil {
		t.Errorf("今までの招待 = (%+v, %v)、取り消されていることを期待", previous, err)
	}
	count, err := repository.NewInvitationRedemptionRepository(db).CountByInviterUserID(ctx, user.ID)
	if err != nil || count != 1 {
		t.Errorf("招待者の使用の件数 = (%d, %v)、1件を期待", count, err)
	}
}

// TestRecreateInvitationUsecase_Execute_Full は、人数の上限に達していれば、今の招待を取り消さず新しい招待も作らないことを検証する。
func TestRecreateInvitationUsecase_Execute_Full(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	for range model.InviterRedemptionLimit {
		testutil.NewInvitationRedemptionBuilder(t, db, currentID).Build()
	}

	output, err := newRecreateInvitationUsecase().Execute(ctx, usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: currentID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Invitation != nil || output.Recreated {
		t.Errorf("招待 = %+v、作り直した = %v、作り直さずにnilを返すことを期待", output.Invitation, output.Recreated)
	}

	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || current == nil || current.ID != currentID {
		t.Errorf("取り消していない招待 = (%+v, %v)、今までの招待 %v のままを期待", current, err, currentID)
	}
}

// TestRecreateInvitationUsecase_Execute_Concurrent は、作り直しが同時に届いても (二重送信)、
// 取り消しと作成が競合してエラーにならず、取り消していない招待が1本に収まることを検証する。
func TestRecreateInvitationUsecase_Execute_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	current := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()

	const requests = 4
	uc := newRecreateInvitationUsecase()
	ids := make([]model.InvitationID, requests)
	recreated := make([]bool, requests)
	errs := make([]error, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			output, err := uc.Execute(ctx, usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: current})
			errs[i] = err
			if err == nil && output.Invitation != nil {
				ids[i] = output.Invitation.ID
				recreated[i] = output.Recreated
			}
		})
	}
	wg.Wait()

	var recreatedCount int
	for i := range requests {
		if errs[i] != nil {
			t.Fatalf("%d件目のExecute()のエラー = %v", i, errs[i])
		}
		if ids[i] == (model.InvitationID{}) {
			t.Fatalf("%d件目の招待 = nil、作り直した招待を期待", i)
		}
		if recreated[i] {
			recreatedCount++
		}
		if ids[i] == current {
			t.Errorf("%d件目の招待 = %v、作り直す前の招待とは別の招待を期待", i, ids[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("%d件目の招待 = %v、1件目と同じ招待 %v を期待", i, ids[i], ids[0])
		}
	}

	if recreatedCount != 1 {
		t.Errorf("作り直したことを示した件数 = %d、1件を期待", recreatedCount)
	}
	previous, err := repository.NewInvitationRepository(db).FindByID(ctx, current)
	if err != nil || previous == nil || previous.RevokedAt == nil {
		t.Errorf("作り直す前の招待 = (%+v, %v)、取り消されていることを期待", previous, err)
	}

	var unrevokedCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM invitations WHERE inviter_user_id = $1 AND revoked_at IS NULL", user.ID.String()).Scan(&unrevokedCount); err != nil || unrevokedCount != 1 {
		t.Errorf("取り消していない招待の数 = (%d, %v)、1本を期待", unrevokedCount, err)
	}
}

// TestRecreateInvitationUsecase_Execute_Duplicate は、最初の作り直しが完了した後に同じフォームを再送しても、
// 新しく作られた招待を取り消さず、作り直したことを示さずに同じ招待を返すことを検証する。
func TestRecreateInvitationUsecase_Execute_Duplicate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	uc := newRecreateInvitationUsecase()
	input := usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: currentID}

	first, err := uc.Execute(ctx, input)
	if err != nil || first == nil || first.Invitation == nil {
		t.Fatalf("1件目の招待 = (%+v, %v)、新しい招待を期待", first, err)
	}
	second, err := uc.Execute(ctx, input)
	if err != nil || second == nil || second.Invitation == nil {
		t.Fatalf("2件目の招待 = (%+v, %v)、1件目の招待を期待", second, err)
	}
	if second.Invitation.ID != first.Invitation.ID {
		t.Errorf("2件目の招待 = %v、1件目の招待 %v を期待", second.Invitation.ID, first.Invitation.ID)
	}
	if !first.Recreated || second.Recreated {
		t.Errorf("作り直した = (1件目 %v, 2件目 %v)、1件目だけが作り直したことを期待", first.Recreated, second.Recreated)
	}
	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || current == nil || current.ID != first.Invitation.ID {
		t.Errorf("使える招待 = (%+v, %v)、1件目の招待を期待", current, err)
	}
}

// TestRecreateInvitationUsecase_Execute_OtherInviter は、別の招待者の招待IDを使っても取り消せないことを検証する。
func TestRecreateInvitationUsecase_Execute_OtherInviter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	otherUser := inviter(t, model.DefaultTimeZone)
	otherID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(otherUser.ID).Build()

	_, err := newRecreateInvitationUsecase().Execute(ctx, usecase.RecreateInvitationInput{Inviter: user, CurrentInvitationID: otherID})
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeForbidden {
		t.Errorf("別の招待者の招待でのエラー = %v、Forbiddenを期待", err)
	}
	otherInvitation, err := repository.NewInvitationRepository(db).FindByID(ctx, otherID)
	if err != nil || otherInvitation == nil || otherInvitation.RevokedAt != nil {
		t.Errorf("別の招待者の招待 = (%+v, %v)、取り消されていないことを期待", otherInvitation, err)
	}
}
