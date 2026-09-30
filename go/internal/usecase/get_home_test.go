package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetHomeUsecase_Execute は、ユーザーのリストごとのアイテムの数量の合計と、
// 交換場所の駅を選んでいるかと、マッチ候補の人数を返すことを検証する。
func TestGetHomeUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := usecase.NewGetHomeUsecase(repository.NewItemRepository(db).WithTx(tx), repository.NewUserRepository(db).WithTx(tx), repository.NewUserStationRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	categoryID := newEventCategoryID(t, tx)
	giveGoodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	wantGoodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	testutil.NewItemBuilder(t, tx, userID, giveGoodsID).WithQuantity(4).Build()
	testutil.NewItemBuilder(t, tx, userID, wantGoodsID).WithKind(model.ItemKindWant).Build()

	output, err := uc.Execute(t.Context(), usecase.GetHomeInput{UserID: userID})
	if err != nil || output.Quantities != (model.ItemQuantities{Give: 4, Want: 1}) || output.HasPlaces || output.MatchCount != 0 {
		t.Fatalf("Execute() = (%+v, %v)、譲れる4・ほしい1で交換場所なし・候補0人を期待", output, err)
	}

	stationID := testutil.NewStationBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, userID, stationID).Build()
	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, partnerID, stationID).Build()
	testutil.NewItemBuilder(t, tx, partnerID, giveGoodsID).WithKind(model.ItemKindWant).Build()
	testutil.NewItemBuilder(t, tx, partnerID, wantGoodsID).Build()
	output, err = uc.Execute(t.Context(), usecase.GetHomeInput{UserID: userID})
	if err != nil || !output.HasPlaces || output.MatchCount != 1 {
		t.Errorf("Execute() = (%+v, %v)、交換場所ありで候補1人を期待", output, err)
	}
}
