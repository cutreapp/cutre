package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
)

// TestMessageConsentRepository_Create は、今の時刻で同意した、やめていない記録を作ることを検証する。
func TestMessageConsentRepository_Create(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewMessageConsentRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()

	consent, err := repo.Create(context.Background(), userID, model.CurrentMessageConsentVersion)
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if consent.UserID != userID || consent.Version != model.CurrentMessageConsentVersion || consent.WithdrawnAt != nil || consent.AgreedAt.IsZero() {
		t.Errorf("作った同意 = %+v、入力のユーザーと今の版の、やめていない同意を期待", consent)
	}
}

// TestMessageConsentRepository_FindLatestByUserID は、ユーザーの記録のうち最後に同意したものを返し、
// 記録が無いときは (nil, nil) を返すことを検証する。
func TestMessageConsentRepository_FindLatestByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewMessageConsentRepository(db).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()
	now := time.Now()
	testutil.NewMessageConsentBuilder(t, tx, userID).WithAgreedAt(now.Add(-2 * time.Hour)).WithWithdrawnAt(now.Add(-time.Hour)).Build()
	latestID := testutil.NewMessageConsentBuilder(t, tx, userID).WithAgreedAt(now).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, otherUserID).WithAgreedAt(now.Add(time.Hour)).Build()

	found, err := repo.FindLatestByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindLatestByUserID()のエラー = %v", err)
	}
	if found == nil || found.ID != latestID {
		t.Errorf("最新の同意 = %+v、ID %s を期待", found, latestID)
	}

	withoutConsent := testutil.NewUserBuilder(t, tx).Build()
	if got, err := repo.FindLatestByUserID(ctx, withoutConsent); err != nil || got != nil {
		t.Errorf("記録の無いユーザーのFindLatestByUserID() = (%+v, %v)、期待値 = (nil, nil)", got, err)
	}
}

// TestMessageConsentRepository_WithdrawByUserID は、ユーザーのやめていない同意をすべてやめたことにし、
// 他のユーザーの同意には触れず、やめていない同意が無いときはfalseを返すことを検証する。
func TestMessageConsentRepository_WithdrawByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewMessageConsentRepository(db).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, userID).WithVersion(model.CurrentMessageConsentVersion - 1).WithAgreedAt(time.Now().Add(-time.Hour)).Build()
	testutil.NewMessageConsentBuilder(t, tx, userID).Build()
	otherUserID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewMessageConsentBuilder(t, tx, otherUserID).Build()

	if ok, err := repo.WithdrawByUserID(ctx, userID); err != nil || !ok {
		t.Fatalf("WithdrawByUserID() = (%t, %v)、期待値 = (true, nil)", ok, err)
	}
	var remaining int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM message_consents WHERE user_id = $1 AND withdrawn_at IS NULL`, userID.String()).Scan(&remaining); err != nil {
		t.Fatalf("やめていない同意の数の取得のエラー = %v", err)
	}
	if remaining != 0 {
		t.Errorf("やめていない同意の数 = %d、期待値 = 0", remaining)
	}
	if other, err := repo.FindLatestByUserID(ctx, otherUserID); err != nil || other == nil || other.WithdrawnAt != nil {
		t.Errorf("他のユーザーの同意 = (%+v, %v)、やめていない同意を期待", other, err)
	}

	if ok, err := repo.WithdrawByUserID(ctx, userID); err != nil || ok {
		t.Errorf("2回目のWithdrawByUserID() = (%t, %v)、期待値 = (false, nil)", ok, err)
	}
}
