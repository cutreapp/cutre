package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newPrepareInvitationUsecase はテスト用のデータベースに直接書き込む PrepareInvitationUsecase を組み立てる。
// UseCaseが自分でトランザクションを開くため、テストのトランザクションでは包まない。
func newPrepareInvitationUsecase() *usecase.PrepareInvitationUsecase {
	db := testutil.GetTestDB()
	return usecase.NewPrepareInvitationUsecase(db, repository.NewUserRepository(db), repository.NewInvitationRepository(db), repository.NewInvitationRedemptionRepository(db))
}

// inviter は招待者になるユーザーを作って返す。
func inviter(t *testing.T, timeZone string) *model.User {
	t.Helper()

	db := testutil.GetTestDB()
	id := testutil.NewUserBuilder(t, db).WithTimeZone(timeZone).Build()
	user, err := repository.NewUserRepository(db).FindByID(context.Background(), id)
	if err != nil || user == nil {
		t.Fatalf("招待者の取得 = (%v, %v)、ユーザーを期待", user, err)
	}

	return user
}

// TestPrepareInvitationUsecase_Execute_Create は、招待を持たないユーザーに、
// 招待者のタイムゾーンの日付で期限を決めた招待を作ることを検証する。
func TestPrepareInvitationUsecase_Execute_Create(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := inviter(t, "America/New_York")

	output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	invitation := output.Invitation
	if invitation == nil || invitation.InviterUserID == nil || *invitation.InviterUserID != user.ID || invitation.Token == "" {
		t.Fatalf("招待 = %+v、招待者 %v のトークン付きの招待を期待", invitation, user.ID)
	}
	loc, _ := time.LoadLocation("America/New_York")
	if want := model.UserInvitationExpiresAt(invitation.CreatedAt, loc); !invitation.ExpiresAt.Equal(want) {
		t.Errorf("有効期限 = %v、期待値 = %v", invitation.ExpiresAt, want)
	}
}

// TestPrepareInvitationUsecase_Execute_Usable は、使える招待があれば作らずにそれを返すことを検証する。
func TestPrepareInvitationUsecase_Execute_Usable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	revokedID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithRevokedAt(time.Now()).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, revokedID).Build()
	currentID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, currentID).Build()

	output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Invitation == nil || output.Invitation.ID != currentID {
		t.Errorf("招待 = %+v、使える招待 %v を期待", output.Invitation, currentID)
	}
}

// TestPrepareInvitationUsecase_Execute_Expired は、期限の切れた招待を取り消して新しい招待を作ることを検証する。
func TestPrepareInvitationUsecase_Execute_Expired(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	expiredID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Invitation == nil || output.Invitation.ID == expiredID || !output.Invitation.IsUsable(time.Now(), 0) {
		t.Errorf("招待 = %+v、期限の切れた招待とは別の使える招待を期待", output.Invitation)
	}

	expired, err := repository.NewInvitationRepository(db).FindByID(ctx, expiredID)
	if err != nil || expired == nil || expired.RevokedAt == nil {
		t.Errorf("期限の切れた招待 = (%+v, %v)、取り消されていることを期待", expired, err)
	}
}

// TestPrepareInvitationUsecase_Execute_Full は、取り消した招待の使用も含めて人数の上限に達していれば、
// 期限の切れた招待があっても新しい招待を作らないことを検証する。
func TestPrepareInvitationUsecase_Execute_Full(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	revokedID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithRevokedAt(time.Now()).Build()
	testutil.NewInvitationRedemptionBuilder(t, db, revokedID).Build()
	expiredID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()
	for range model.InviterRedemptionLimit - 1 {
		testutil.NewInvitationRedemptionBuilder(t, db, expiredID).Build()
	}

	output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Invitation != nil {
		t.Errorf("招待 = %+v、nilを期待", output.Invitation)
	}

	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || current == nil || current.ID != expiredID {
		t.Errorf("取り消していない招待 = (%+v, %v)、期限の切れた招待のまま (作り直さない) を期待", current, err)
	}
}

// TestPrepareInvitationUsecase_Execute_Concurrent は、同じユーザーが同時に開いても、
// 期限の切れた招待の作り直しが1本に収束し、どのリクエストも同じ招待を返すことを検証する。
func TestPrepareInvitationUsecase_Execute_Concurrent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	const requests = 4
	uc := newPrepareInvitationUsecase()
	ids := make([]model.InvitationID, requests)
	errs := make([]error, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			output, err := uc.Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
			errs[i] = err
			if err == nil && output.Invitation != nil {
				ids[i] = output.Invitation.ID
			}
		})
	}
	wg.Wait()

	for i := range requests {
		if errs[i] != nil {
			t.Fatalf("%d件目のExecute()のエラー = %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("%d件目の招待 = %v、1件目と同じ招待 %v を期待", i, ids[i], ids[0])
		}
	}
}

// TestPrepareInvitationUsecase_Execute_LastRedemption は、期限切れの招待を作り直す処理が
// 招待行のロックを待つ間に最後の枠が埋まった場合、新しい招待を作らないことを検証する。
func TestPrepareInvitationUsecase_Execute_LastRedemption(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	expiredID := testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()
	for range model.InviterRedemptionLimit - 1 {
		testutil.NewInvitationRedemptionBuilder(t, db, expiredID).Build()
	}

	lockTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("最後の枠を埋めるトランザクションの開始に失敗: %v", err)
	}
	defer func() { _ = lockTx.Rollback() }()
	var lockedID uuid.UUID
	if err := lockTx.QueryRowContext(ctx, "SELECT id FROM invitations WHERE id = $1 FOR UPDATE", uuid.UUID(expiredID)).Scan(&lockedID); err != nil {
		t.Fatalf("招待のロックに失敗: %v", err)
	}
	var lockPID int
	if err := lockTx.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&lockPID); err != nil {
		t.Fatalf("ロックを保持する接続のPIDの取得に失敗: %v", err)
	}

	type result struct {
		output *usecase.PrepareInvitationOutput
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
		done <- result{output: output, err: err}
	}()

	// ロック待ちを観測してから5人目を登録し、最初の件数取得後の再判定を確実に通す。
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case got := <-done:
			t.Fatalf("ロックの解放前に招待の準備が終わった: %v", got.err)
		default:
		}
		var waiting bool
		if err := db.QueryRowContext(ctx,
			`SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
				  AND query LIKE '%FOR UPDATE%'
			)`, lockPID,
		).Scan(&waiting); err != nil {
			t.Fatalf("ロック待ちの確認に失敗: %v", err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("招待の準備が招待行のロックを待たなかった")
		}
		time.Sleep(10 * time.Millisecond)
	}

	testutil.NewInvitationRedemptionBuilder(t, lockTx, expiredID).Build()
	if err := lockTx.Commit(); err != nil {
		t.Fatalf("最後の枠の使用を確定できなかった: %v", err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("Execute()のエラー = %v", got.err)
		}
		if got.output.Invitation != nil {
			t.Errorf("招待 = %+v、nilを期待", got.output.Invitation)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("最後の枠の使用を確定した後も招待の準備が終わらない")
	}

	current, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || current == nil || current.ID != expiredID {
		t.Errorf("取り消していない招待 = (%+v, %v)、期限の切れた招待のままを期待", current, err)
	}
}

// TestPrepareInvitationUsecase_Execute_WithdrawnInviter は、画面を開いた後に招待者が退会していたとき、
// 招待を作らず、失敗にもせずに招待が無いものとして返すことを検証する。
func TestPrepareInvitationUsecase_Execute_WithdrawnInviter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := inviter(t, model.DefaultTimeZone)
	withdrawn, err := repository.NewUserRepository(testutil.GetTestDB()).Withdraw(ctx, user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID))
	if err != nil || !withdrawn {
		t.Fatalf("Withdraw() = (%t, %v)、(true, nil) を期待", withdrawn, err)
	}

	output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
	if err != nil || output.Invitation != nil {
		t.Fatalf("Execute() = (%+v, %v)、招待の無い結果を期待", output, err)
	}
	unrevoked, err := repository.NewInvitationRepository(testutil.GetTestDB()).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || unrevoked != nil {
		t.Errorf("取り消していない招待 = (%+v, %v)、(nil, nil) を期待", unrevoked, err)
	}
}

// TestPrepareInvitationUsecase_Execute_DuringWithdrawal は、招待者の退会がコミットされる前に期限切れの招待を作り直そうとしたとき、
// 退会のコミットを待ってから作らずに終わり、退会したユーザーに取り消していない招待を残さないことを検証する。
// 退会と同じくユーザーの行を招待の行より先にロックするため、デッドロックもしない。
func TestPrepareInvitationUsecase_Execute_DuringWithdrawal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testutil.GetTestDB()
	user := inviter(t, model.DefaultTimeZone)
	testutil.NewInvitationBuilder(t, db).WithInviterUserID(user.ID).WithExpiresAt(time.Now().Add(-time.Minute)).Build()

	// DeleteAccountUsecase と同じ順で、ユーザーの行を更新してから招待を取り消す。
	withdrawalTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("トランザクションの開始のエラー = %v", err)
	}
	defer func() { _ = withdrawalTx.Rollback() }()
	withdrawn, err := repository.NewUserRepository(db).WithTx(withdrawalTx).Withdraw(ctx, user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID))
	if err != nil || !withdrawn {
		t.Fatalf("Withdraw() = (%t, %v)、(true, nil) を期待", withdrawn, err)
	}

	type result struct {
		output *usecase.PrepareInvitationOutput
		err    error
	}
	done := make(chan result, 1)
	go func() {
		output, err := newPrepareInvitationUsecase().Execute(ctx, usecase.PrepareInvitationInput{Inviter: user})
		done <- result{output: output, err: err}
	}()

	select {
	case r := <-done:
		t.Fatalf("退会のコミット前に招待の用意が終わった = (%+v, %v)、退会のコミットを待つことを期待", r.output, r.err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := repository.NewInvitationRepository(db).WithTx(withdrawalTx).RevokeUnrevokedByInviterUserID(ctx, user.ID); err != nil {
		t.Fatalf("RevokeUnrevokedByInviterUserID()のエラー = %v", err)
	}
	if err := withdrawalTx.Commit(); err != nil {
		t.Fatalf("退会のコミットのエラー = %v", err)
	}

	r := <-done
	if r.err != nil || r.output.Invitation != nil {
		t.Errorf("Execute() = (%+v, %v)、招待の無い結果を期待", r.output, r.err)
	}
	unrevoked, err := repository.NewInvitationRepository(db).FindUnrevokedByInviterUserID(ctx, user.ID)
	if err != nil || unrevoked != nil {
		t.Errorf("取り消していない招待 = (%+v, %v)、(nil, nil) を期待", unrevoked, err)
	}
}
