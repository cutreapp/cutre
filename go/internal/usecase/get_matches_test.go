package usecase_test

import (
	"database/sql"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// newGetMatchesUsecase はテストのトランザクションで読む GetMatchesUsecase を組み立てる。
func newGetMatchesUsecase(db *sql.DB, tx *sql.Tx) *usecase.GetMatchesUsecase {
	return usecase.NewGetMatchesUsecase(
		repository.NewEventCategoryRepository(db).WithTx(tx),
		repository.NewGoodsRepository(db).WithTx(tx),
		repository.NewItemRepository(db).WithTx(tx),
		repository.NewStationRepository(db).WithTx(tx),
		repository.NewUserRepository(db).WithTx(tx),
		repository.NewUserStationRepository(db).WithTx(tx),
	)
}

// TestGetMatchesUsecase_Execute は、マッチ候補ごとに、もらえるもの・渡せるものと交換場所を、グッズとカテゴリーと一緒に返すことを検証する。
func TestGetMatchesUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetMatchesUsecase(db, tx)
	categoryID := newEventCategoryID(t, tx)
	wantedGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	offeredGoods := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	stationID := testutil.NewStationBuilder(t, tx).WithPrefectureCode(13).Build()

	userID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, userID, stationID).Build()
	testutil.NewItemBuilder(t, tx, userID, wantedGoods).WithKind(model.ItemKindWant).WithQuantity(2).Build()
	myItemID := testutil.NewItemBuilder(t, tx, userID, offeredGoods).Build()

	partnerID := testutil.NewUserBuilder(t, tx).Build()
	testutil.NewUserStationBuilder(t, tx, partnerID, stationID).Build()
	partnerItemID := testutil.NewItemBuilder(t, tx, partnerID, wantedGoods).WithQuantity(3).Build()
	testutil.NewItemBuilder(t, tx, partnerID, offeredGoods).WithKind(model.ItemKindWant).Build()

	output, err := uc.Execute(t.Context(), usecase.GetMatchesInput{UserID: userID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Candidates) != 1 || output.Candidates[0].ID != partnerID || !output.HasPlaces {
		t.Fatalf("Execute() = %+v、交換場所ありで候補1人を期待", output)
	}
	match := output.Matches[partnerID]
	if len(match.Receivable) != 1 || match.Receivable[0].Item.ID != partnerItemID || match.Receivable[0].Quantity != 2 {
		t.Errorf("もらえるもの = %+v、相手のアイテムを2点を期待", match.Receivable)
	}
	if len(match.Givable) != 1 || match.Givable[0].Item.ID != myItemID {
		t.Errorf("渡せるもの = %+v、自分のアイテムを期待", match.Givable)
	}
	if got := output.Stations[partnerID]; len(got) != 1 || got[0].ID != stationID {
		t.Errorf("候補の交換場所 = %v、1駅を期待", got)
	}
	if output.Goods[wantedGoods] == nil || output.Goods[offeredGoods] == nil || output.EventCategories[categoryID] == nil {
		t.Errorf("Goods = %v, EventCategories = %v、交換できるアイテムのグッズとカテゴリーを期待", output.Goods, output.EventCategories)
	}
}

// TestGetMatchesUsecase_Execute_NoPlaces は、交換場所を選んでいないユーザーには候補を出さず、交換場所が無いことを返すことを検証する。
func TestGetMatchesUsecase_Execute_NoPlaces(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newGetMatchesUsecase(db, tx)

	output, err := uc.Execute(t.Context(), usecase.GetMatchesInput{UserID: testutil.NewUserBuilder(t, tx).Build()})
	if err != nil || len(output.Candidates) != 0 || output.HasPlaces {
		t.Errorf("Execute() = (%+v, %v)、交換場所なしで候補0人を期待", output, err)
	}
}
