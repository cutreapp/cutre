package repository_test

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestEmailConfirmationRepository_Create は、確認を未確認・失敗0回の状態で作って返すことを検証する。
func TestEmailConfirmationRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("email-confirmation")
	expiresAt := model.EmailConfirmationExpiresAt(time.Now()).Truncate(time.Microsecond)

	confirmation, err := repo.Create(context.Background(), repository.CreateEmailConfirmationInput{
		Email:     email,
		Code:      "012345",
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if confirmation.Email != email || confirmation.Code != "012345" || !confirmation.ExpiresAt.Equal(expiresAt) {
		t.Errorf("確認 = %+v、入力のメールアドレス・コード・有効期限を期待", confirmation)
	}
	if confirmation.FailedAttemptsCount != 0 || confirmation.ConfirmedAt != nil {
		t.Errorf("失敗の回数・確認の時刻 = (%d, %v)、(0, nil)を期待", confirmation.FailedAttemptsCount, confirmation.ConfirmedAt)
	}
}

// TestEmailConfirmationRepository_FindUnconfirmedByID は、確認済みでない確認だけを返し、
// 期限切れや誤入力の上限に達した確認も再送のために返すことを検証する。
func TestEmailConfirmationRepository_FindUnconfirmedByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	tests := []struct {
		name  string
		id    model.EmailConfirmationID
		found bool
	}{
		{name: "未確認", id: testutil.NewEmailConfirmationBuilder(t, tx).Build(), found: true},
		{name: "期限切れ", id: testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(time.Now().Add(-time.Minute)).Build(), found: true},
		{name: "誤入力の上限", id: testutil.NewEmailConfirmationBuilder(t, tx).WithFailedAttemptsCount(model.EmailConfirmationMaxFailedAttempts).Build(), found: true},
		{name: "確認済み", id: testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build()},
		{name: "存在しない", id: model.EmailConfirmationID(uuid.New())},
	}
	for _, tt := range tests {
		confirmation, err := repo.FindUnconfirmedByID(context.Background(), tt.id)
		if err != nil {
			t.Fatalf("%s: FindUnconfirmedByID()のエラー = %v", tt.name, err)
		}
		if (confirmation != nil) != tt.found {
			t.Errorf("%s: 確認 = %+v、見つかる = %t を期待", tt.name, confirmation, tt.found)
		}
	}
}

// TestEmailConfirmationRepository_Verify は、一致するコードで確認済みにし、違うコードで誤入力の回数を増やし、
// 照合できない確認 (期限切れ・誤入力の上限・確認済み) は更新せずnilを返すことを検証する。
func TestEmailConfirmationRepository_Verify(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)
	ctx := context.Background()

	matched, err := repo.Verify(ctx, testutil.NewEmailConfirmationBuilder(t, tx).WithCode("654321").Build(), "654321")
	if err != nil {
		t.Fatalf("Verify()のエラー = %v", err)
	}
	if matched == nil || matched.ConfirmedAt == nil || matched.FailedAttemptsCount != 0 {
		t.Errorf("一致したときの確認 = %+v、確認済みで誤入力0回を期待", matched)
	}

	mismatched, err := repo.Verify(ctx, testutil.NewEmailConfirmationBuilder(t, tx).WithCode("654321").WithFailedAttemptsCount(2).Build(), "000000")
	if err != nil {
		t.Fatalf("Verify()のエラー = %v", err)
	}
	if mismatched == nil || mismatched.ConfirmedAt != nil || mismatched.FailedAttemptsCount != 3 {
		t.Errorf("一致しないときの確認 = %+v、未確認で誤入力3回を期待", mismatched)
	}

	unusable := map[string]model.EmailConfirmationID{
		"期限切れ":   testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(time.Now().Add(-time.Minute)).Build(),
		"誤入力の上限": testutil.NewEmailConfirmationBuilder(t, tx).WithFailedAttemptsCount(model.EmailConfirmationMaxFailedAttempts).Build(),
		"確認済み":   testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build(),
		"存在しない":  model.EmailConfirmationID(uuid.New()),
	}
	for name, id := range unusable {
		// 正しいコード (ビルダーの既定) を送っても照合しない。
		confirmation, err := repo.Verify(ctx, id, "123456")
		if err != nil {
			t.Fatalf("%s: Verify()のエラー = %v", name, err)
		}
		if confirmation != nil {
			t.Errorf("%s: 確認 = %+v、nilを期待", name, confirmation)
		}
	}
}

// TestEmailConfirmationRepository_Verify_Concurrent は、同時に送られた誤ったコードが
// 誤入力の上限を超えて照合されないことを検証する。
func TestEmailConfirmationRepository_Verify_Concurrent(t *testing.T) {
	t.Parallel()

	// 別々の接続から同時に照合させるため、テストのトランザクションでは包まない。
	db := testutil.GetTestDB()
	repo := repository.NewEmailConfirmationRepository(db)
	id := testutil.NewEmailConfirmationBuilder(t, db).Build()

	const attempts = 3 * model.EmailConfirmationMaxFailedAttempts
	var wg sync.WaitGroup
	var verified atomic.Int32
	for range attempts {
		wg.Go(func() {
			confirmation, err := repo.Verify(context.Background(), id, "000000")
			if err != nil {
				t.Errorf("Verify()のエラー = %v", err)
				return
			}
			if confirmation != nil {
				verified.Add(1)
			}
		})
	}
	wg.Wait()

	if got := verified.Load(); got != model.EmailConfirmationMaxFailedAttempts {
		t.Errorf("照合された回数 = %d、期待値 = %d", got, model.EmailConfirmationMaxFailedAttempts)
	}
}

// TestEmailConfirmationRepository_FindConfirmedByID は、確認済みの確認だけを返すことを検証する。
func TestEmailConfirmationRepository_FindConfirmedByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	tests := []struct {
		name  string
		id    model.EmailConfirmationID
		found bool
	}{
		{name: "確認済み", id: testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build(), found: true},
		{name: "未確認", id: testutil.NewEmailConfirmationBuilder(t, tx).Build()},
		{name: "無いID", id: model.EmailConfirmationID(uuid.New())},
	}
	for _, tt := range tests {
		confirmation, err := repo.FindConfirmedByID(context.Background(), tt.id)
		if err != nil {
			t.Fatalf("%s: FindConfirmedByID()のエラー = %v", tt.name, err)
		}
		if found := confirmation != nil; found != tt.found {
			t.Errorf("%s: 見つかった = %t、期待値 = %t", tt.name, found, tt.found)
		}
	}
}

// TestEmailConfirmationRepository_DeleteConfirmed は、確認済みの確認だけを削除し、2回目以降はfalseを返すことを検証する。
func TestEmailConfirmationRepository_DeleteConfirmed(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	confirmedID := testutil.NewEmailConfirmationBuilder(t, tx).WithConfirmedAt(time.Now()).Build()
	unconfirmedID := testutil.NewEmailConfirmationBuilder(t, tx).Build()

	if deleted, err := repo.DeleteConfirmed(ctx, confirmedID); err != nil || !deleted {
		t.Fatalf("DeleteConfirmed(確認済み) = (%t, %v)、(true, nil)を期待", deleted, err)
	}
	if found, err := repo.FindConfirmedByID(ctx, confirmedID); err != nil || found != nil {
		t.Errorf("削除後のFindConfirmedByID() = (%+v, %v)、(nil, nil)を期待", found, err)
	}
	if deleted, err := repo.DeleteConfirmed(ctx, confirmedID); err != nil || deleted {
		t.Errorf("DeleteConfirmed(削除済み) = (%t, %v)、(false, nil)を期待", deleted, err)
	}
	if deleted, err := repo.DeleteConfirmed(ctx, unconfirmedID); err != nil || deleted {
		t.Errorf("DeleteConfirmed(未確認) = (%t, %v)、(false, nil)を期待", deleted, err)
	}
}

// TestEmailConfirmationRepository_DeleteExpired は、指定した時刻までに有効期限が切れた確認を、確認済みかどうかによらず消し、
// それより後に有効期限が来る確認を残すことを検証する。
func TestEmailConfirmationRepository_DeleteExpired(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	before := time.Now().UTC()
	expiredID := testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(before.Add(-time.Minute)).Build()
	expiredConfirmedID := testutil.NewEmailConfirmationBuilder(t, tx).
		WithExpiresAt(before.Add(-time.Minute)).
		WithConfirmedAt(before.Add(-10 * time.Minute)).
		Build()
	liveID := testutil.NewEmailConfirmationBuilder(t, tx).WithExpiresAt(before.Add(time.Minute)).Build()

	if err := repo.DeleteExpired(ctx, before); err != nil {
		t.Fatalf("DeleteExpired()のエラー = %v", err)
	}

	tests := []struct {
		name string
		id   model.EmailConfirmationID
		want bool
	}{
		{name: "期限切れの未確認の確認", id: expiredID, want: false},
		{name: "期限切れの確認済みの確認", id: expiredConfirmedID, want: false},
		{name: "期限がまだ来ていない確認", id: liveID, want: true},
	}
	for _, tt := range tests {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM email_confirmations WHERE id = $1)", uuid.UUID(tt.id)).Scan(&exists); err != nil {
			t.Fatalf("確認の存在確認のエラー = %v", err)
		}
		if exists != tt.want {
			t.Errorf("%sが残っているか = %t、期待値 = %t", tt.name, exists, tt.want)
		}
	}
}

// TestEmailConfirmationRepository_DeleteByEmail は、メールアドレスへの確認を大文字小文字を無視して確認済みかどうかによらず消し、
// ほかのアドレスへの確認を残すことを検証する。
func TestEmailConfirmationRepository_DeleteByEmail(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(db).WithTx(tx)

	email := testutil.UniqueEmail("delete-by-email")
	unconfirmedID := testutil.NewEmailConfirmationBuilder(t, tx).WithEmail(email).Build()
	confirmedID := testutil.NewEmailConfirmationBuilder(t, tx).WithEmail(strings.ToUpper(email)).WithConfirmedAt(time.Now()).Build()
	otherID := testutil.NewEmailConfirmationBuilder(t, tx).WithEmail(testutil.UniqueEmail("delete-by-email-other")).Build()

	if err := repo.DeleteByEmail(ctx, email); err != nil {
		t.Fatalf("DeleteByEmail()のエラー = %v", err)
	}

	tests := []struct {
		name string
		id   model.EmailConfirmationID
		want bool
	}{
		{name: "未確認の確認", id: unconfirmedID, want: false},
		{name: "大文字の同じアドレスへの確認済みの確認", id: confirmedID, want: false},
		{name: "ほかのアドレスへの確認", id: otherID, want: true},
	}
	for _, tt := range tests {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM email_confirmations WHERE id = $1)", uuid.UUID(tt.id)).Scan(&exists); err != nil {
			t.Fatalf("確認の存在確認のエラー = %v", err)
		}
		if exists != tt.want {
			t.Errorf("%sが残っているか = %t、期待値 = %t", tt.name, exists, tt.want)
		}
	}
}
