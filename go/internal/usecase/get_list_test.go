package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetListUsecase_Execute は、指定したリストのアイテムと、そのグッズ・カテゴリー・イベント (アーカイブしたものを含む)、
// リストごとの数量の合計を返すことを検証する。
func TestGetListUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetListUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).WithArchived("終わったため").Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithQuantity(2).Build()
	testutil.NewItemBuilder(t, tx, userID, testutil.NewGoodsBuilder(t, tx, categoryID).Build()).WithKind(model.ItemKindWant).WithQuantity(3).Build()

	output, err := uc.Execute(t.Context(), usecase.GetListInput{UserID: userID, Kind: model.ItemKindGive})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].ID != itemID {
		t.Fatalf("アイテム = %+v、譲れるリストの %s だけを期待", output.Items, itemID)
	}
	if output.Goods[goodsID] == nil || output.EventCategories[categoryID] == nil || output.Events[eventID] == nil {
		t.Errorf("マスタ = (%v, %v, %v)、アイテムのグッズ・カテゴリー・イベントを期待", output.Goods, output.EventCategories, output.Events)
	}
	if output.Quantities != (model.ItemQuantities{Give: 2, Want: 3}) {
		t.Errorf("数量の合計 = %+v、譲れる2・ほしい3を期待", output.Quantities)
	}
}

// TestGetListUsecase_Execute_Empty は、アイテムが無いリストでは、マスタを引かずに空の結果を返すことを検証する。
func TestGetListUsecase_Execute_Empty(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetListUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))

	output, err := uc.Execute(t.Context(), usecase.GetListInput{UserID: testutil.NewUserBuilder(t, tx).Build(), Kind: model.ItemKindWant})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.Items) != 0 || len(output.Goods) != 0 || output.Quantities != (model.ItemQuantities{}) {
		t.Errorf("結果 = %+v、空を期待", output)
	}
}
