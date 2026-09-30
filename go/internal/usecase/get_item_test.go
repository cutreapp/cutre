package usecase_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestGetItemUsecase_Execute は、ユーザーのリストにあるアイテムを、アーカイブしたグッズのものも含めて
// そのグッズ・カテゴリー・イベントと一緒に返すことを検証する。
func TestGetItemUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetItemUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	eventID := testutil.NewEventBuilder(t, tx).Build()
	categoryID := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, categoryID).WithArchived("景品から外れたため").Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()

	output, err := uc.Execute(t.Context(), usecase.GetItemInput{UserID: userID, ItemID: itemID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Item.ID != itemID || output.Goods.ID != goodsID || output.EventCategory.ID != categoryID || output.Event.ID != eventID {
		t.Errorf("結果 = (%s, %s, %s, %s)、(%s, %s, %s, %s) を期待", output.Item.ID, output.Goods.ID, output.EventCategory.ID, output.Event.ID, itemID, goodsID, categoryID, eventID)
	}
}

// TestGetItemUsecase_Execute_NotFound は、無いアイテム・外したアイテム・ほかのユーザーのアイテムに
// AppErrCodeResourceNotFound を返すことを検証する。
func TestGetItemUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	eventRepo, categoryRepo, goodsRepo := newGoodsRepos(db, tx)
	uc := usecase.NewGetItemUsecase(eventRepo, categoryRepo, goodsRepo, repository.NewItemRepository(db).WithTx(tx))
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()

	for name, itemID := range map[string]model.ItemID{
		"無いアイテム":       model.ItemID(uuid.New()),
		"外したアイテム":      testutil.NewItemBuilder(t, tx, userID, goodsID).WithRemoved().Build(),
		"ほかのユーザーのアイテム": testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build(),
	} {
		_, err := uc.Execute(t.Context(), usecase.GetItemInput{UserID: userID, ItemID: itemID})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
}
