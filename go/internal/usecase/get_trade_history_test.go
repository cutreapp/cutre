package usecase_test

import (
	"testing"
	"time"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetTradeHistoryUsecase_Execute は、終わった交換だけを、終わった時刻が新しい順に、相手と交換の品と一緒に返すことを検証する。
func TestGetTradeHistoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetTradeHistoryUsecase(
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeItemRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	userID := testutil.NewUserBuilder(t, tx).Build()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	// 退会した相手との交換も返す。
	withdrawnUserID := testutil.NewUserBuilder(t, tx).WithDeletedAt(time.Now()).Build()
	now := time.Now()
	completed := testutil.NewTradeBuilder(t, tx, proposerID, userID).WithStatus(model.TradeStatusCompleted).WithEndedAt(now.Add(-time.Hour)).Build()
	declined := testutil.NewTradeBuilder(t, tx, userID, withdrawnUserID).WithStatus(model.TradeStatusDeclined).WithEndedAt(now).Build()
	testutil.NewTradeBuilder(t, tx, userID, proposerID).Build()
	itemID := testutil.NewItemBuilder(t, tx, proposerID, goodsID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), completed, []model.ItemID{itemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	output, err := uc.Execute(t.Context(), usecase.GetTradeHistoryInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Trades) != 2 || output.Trades[0].ID != declined || output.Trades[1].ID != completed {
		t.Errorf("Trades = %+v、[お断り, 交換できた] を期待", output.Trades)
	}
	if len(output.Partners) != 2 || output.Partners[proposerID] == nil || output.Partners[withdrawnUserID] == nil {
		t.Errorf("Partners = %v、2つの交換の相手 (退会した相手を含む) を期待", output.Partners)
	}
	if items := output.Items[completed]; len(items) != 1 || items[0].ID != itemID || len(output.Items[declined]) != 0 {
		t.Errorf("Items = %v、交換できた交換の品の1点を期待", output.Items)
	}

	output, err = uc.Execute(t.Context(), usecase.GetTradeHistoryInput{UserID: testutil.NewUserBuilder(t, tx).Build()})
	if err != nil || len(output.Trades) != 0 {
		t.Errorf("交換の無いユーザー: (%+v, %v)、空を期待", output, err)
	}
}
