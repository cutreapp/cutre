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

// TestCreateTradeUsecase_ReceiverConsentWithdrawal は、申し込まれた人が同意をやめる操作も、
// 申し込みのコミットを待ってから進行中の交換を確かめることを検証する。
func TestCreateTradeUsecase_ReceiverConsentWithdrawal(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)
	db := testutil.GetTestDB()
	testutil.NewMessageConsentBuilder(t, db, f.receiver.ID).Build()
	ctx, cancel := context.WithTimeout(jaContext(), 5*time.Second)
	defer cancel()

	// アイテムを保持して申し込みを待たせる。申し込みはユーザー2人を先にロックする。
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	itemID := uuid.UUID(f.receiveItemID)
	if err := tx.QueryRowContext(ctx, "SELECT id FROM items WHERE id = $1 FOR NO KEY UPDATE", itemID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}

	created := make(chan error, 1)
	go func() {
		_, err := newCreateTradeUsecase().Execute(ctx, f.input(""))
		created <- err
	}()
	assertBlocked(t, created)

	withdrawn := make(chan error, 1)
	go func() {
		withdrawn <- newWithdrawMessageConsentUsecase().Execute(ctx, usecase.WithdrawMessageConsentInput{UserID: f.receiver.ID})
	}()
	assertBlocked(t, withdrawn)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-created; err != nil {
		t.Fatalf("申し込みのエラー = %v", err)
	}
	if ve := model.AsValidationError(<-withdrawn); ve == nil || len(ve.Global) != 1 {
		t.Errorf("同意をやめる操作のエラー = %v、進行中の交換のValidationErrorを期待", ve)
	}
	consent, err := repository.NewMessageConsentRepository(db).FindLatestByUserID(ctx, f.receiver.ID)
	if err != nil || consent == nil || !consent.IsValid() {
		t.Errorf("申し込まれた人の同意 = (%+v, %v)、有効なままを期待", consent, err)
	}
}

// TestCreateTradeUsecase_ItemRemovalWins は、アイテムを外す操作が先に始まったとき、
// 申し込みがそのコミットを待ってから選べない品として拒否することを検証する。
func TestCreateTradeUsecase_ItemRemovalWins(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)
	db := testutil.GetTestDB()
	ctx, cancel := context.WithTimeout(jaContext(), 5*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	removed, err := repository.NewItemRepository(db).WithTx(tx).RemoveListed(ctx, f.receiveItemID, f.receiver.ID, 0)
	if err != nil || !removed {
		t.Fatalf("アイテムを外す操作 = (%v, %v)、成功を期待", removed, err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := newCreateTradeUsecase().Execute(ctx, f.input(""))
		done <- err
	}()
	assertBlocked(t, done)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if ve := model.AsValidationError(<-done); ve == nil || len(ve.Global) != 1 {
		t.Errorf("申し込みのエラー = %v、選べない品のValidationErrorを期待", ve)
	}
	if exists, err := repository.NewTradeRepository(db).ExistsInProgressByUserID(ctx, f.proposer.ID); err != nil || exists {
		t.Errorf("進行中の交換 = (%v, %v)、無いことを期待", exists, err)
	}
}

// TestCreateTradeUsecase_OppositeProposals は、2人が互いに同時に申し込んでも、
// ユーザー行とアイテム行の取得順が一致してデッドロックしないことを検証する。
func TestCreateTradeUsecase_OppositeProposals(t *testing.T) {
	t.Parallel()

	f := newTradeFixture(t)
	db := testutil.GetTestDB()
	testutil.NewMessageConsentBuilder(t, db, f.receiver.ID).Build()
	ctx, cancel := context.WithTimeout(jaContext(), 5*time.Second)
	defer cancel()
	reverse := usecase.CreateTradeInput{
		ProposerUserID: f.receiver.ID,
		Atname:         f.proposer.Atname,
		ReceiveItemIDs: []string{f.giveItemID.String()},
		GiveItemIDs:    []string{f.receiveItemID.String()},
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, input := range []usecase.CreateTradeInput{f.input(""), reverse} {
		go func() {
			<-start
			_, err := newCreateTradeUsecase().Execute(ctx, input)
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Errorf("同時の申し込みのエラー = %v", err)
		}
	}
}
