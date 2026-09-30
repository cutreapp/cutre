package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetEventUsecase_Execute は、公開中のイベントの公開中のカテゴリーを、カテゴリーごとのグッズの数とユーザーのアイテムの数量と一緒に返し、
// 公開していないイベントには AppErrCodeResourceNotFound を返すことを検証する。
func TestGetEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetEventUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	testutil.NewEventCategoryBuilder(t, tx, eventID).WithArchived("景品から外れたため").Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).Build()
	testutil.NewItemBuilder(t, tx, userID, goodsID).WithKind(model.ItemKindWant).WithQuantity(3).Build()

	output, err := uc.Execute(t.Context(), usecase.GetEventInput{UserID: userID, EventID: eventID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if len(output.EventCategories) != 1 || output.EventCategories[0].ID != categoryID {
		t.Errorf("カテゴリー = %+v、公開中の %s だけを期待", output.EventCategories, categoryID)
	}
	if output.GoodsCounts[categoryID] != 1 || output.Quantities[categoryID] != (model.ItemQuantities{Want: 3}) {
		t.Errorf("グッズの数と数量 = (%d, %+v)、(1, ほしい3) を期待", output.GoodsCounts[categoryID], output.Quantities[categoryID])
	}

	_, err = uc.Execute(t.Context(), usecase.GetEventInput{UserID: userID, EventID: testutil.NewEventBuilder(t, tx).WithDeleted().Build()})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)
}
