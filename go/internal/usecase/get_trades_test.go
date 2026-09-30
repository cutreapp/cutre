package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetTradesUsecase_Execute は、進行中の交換だけを、相手と交換の品と一緒に返し、終わった交換は段階ごとの数で返すことを検証する。
func TestGetTradesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetTradesUsecase(
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewTradeRepository(db).WithTx(tx),
		repository.NewTradeItemRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
	)
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	userID := testutil.NewUserBuilder(t, tx).Build()
	proposerID := testutil.NewUserBuilder(t, tx).Build()
	receiverID := testutil.NewUserBuilder(t, tx).Build()
	received := testutil.NewTradeBuilder(t, tx, proposerID, userID).Build()
	proposed := testutil.NewTradeBuilder(t, tx, userID, receiverID).WithStatus(model.TradeStatusMatched).Build()
	testutil.NewTradeBuilder(t, tx, userID, receiverID).WithStatus(model.TradeStatusWithdrawn).Build()
	itemID := testutil.NewItemBuilder(t, tx, proposerID, goodsID).Build()
	if err := repository.NewTradeItemRepository(db).WithTx(tx).CreateMany(t.Context(), received, []model.ItemID{itemID}); err != nil {
		t.Fatalf("CreateMany()のエラー = %v", err)
	}

	output, err := uc.Execute(t.Context(), usecase.GetTradesInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Trades) != 2 || output.Trades[0].ID != proposed || output.Trades[1].ID != received {
		t.Errorf("Trades = %+v、[申し込んだマッチ成立, 申し込まれた返事待ち] を期待", output.Trades)
	}
	if len(output.Partners) != 2 || output.Partners[proposerID] == nil || output.Partners[receiverID] == nil {
		t.Errorf("Partners = %v、2つの交換の相手を期待", output.Partners)
	}
	if items := output.Items[received]; len(items) != 1 || items[0].ID != itemID || len(output.Items[proposed]) != 0 {
		t.Errorf("Items = %v、申し込まれた交換の品の1点を期待", output.Items)
	}
	if len(output.EndedCounts) != 1 || output.EndedCounts[model.TradeStatusWithdrawn] != 1 {
		t.Errorf("EndedCounts = %v、取り下げの1件を期待", output.EndedCounts)
	}

	output, err = uc.Execute(t.Context(), usecase.GetTradesInput{UserID: testutil.NewUserBuilder(t, tx).Build()})
	if err != nil || len(output.Trades) != 0 || len(output.EndedCounts) != 0 {
		t.Errorf("交換の無いユーザー: (%+v, %v)、空を期待", output, err)
	}
}
