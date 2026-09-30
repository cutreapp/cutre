package usecase_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
)

// TestDeleteItemUsecase_Execute は、アイテムを消さずにリストから外すことと、2回目は外したアイテムとして
// AppErrCodeResourceNotFound を返すことを検証する。
func TestDeleteItemUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	uc := usecase.NewDeleteItemUsecase(itemRepo)
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithQuantity(2).Build()

	output, err := uc.Execute(t.Context(), usecase.DeleteItemInput{UserID: userID, ItemID: itemID})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Item.ID != itemID || output.Item.Kind != model.ItemKindGive {
		t.Errorf("結果のアイテム = %+v、譲れるリストの %s を期待", output.Item, itemID)
	}
	if item, _ := itemRepo.FindByID(context.Background(), itemID); item == nil || item.Status != model.ItemStatusRemoved || item.Quantity != 2 || item.LockVersion != 1 {
		t.Errorf("外したあとのアイテム = %+v、行を残して外した状態を期待", item)
	}

	_, err = uc.Execute(t.Context(), usecase.DeleteItemInput{UserID: userID, ItemID: itemID})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)
}

// TestDeleteItemUsecase_Execute_Conflict は古い版の削除を拒み、アイテムをリストに残すことを検証する。
func TestDeleteItemUsecase_Execute_Conflict(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).Build()
	if updated, err := itemRepo.UpdateListed(t.Context(), itemID, userID, 0, repository.ItemUpdateAttributes{Quantity: 2}); !updated || err != nil {
		t.Fatalf("先の更新 = (%v, %v)、成功を期待", updated, err)
	}

	_, err := usecase.NewDeleteItemUsecase(itemRepo).Execute(t.Context(), usecase.DeleteItemInput{UserID: userID, ItemID: itemID, LockVersion: 0})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
	if item, _ := itemRepo.FindByID(t.Context(), itemID); item.Status != model.ItemStatusListed || item.Quantity != 2 || item.LockVersion != 1 {
		t.Errorf("古い版からの削除後 = %+v、先の変更と版1を維持することを期待", item)
	}
}

// TestDeleteItemUsecase_Execute_OtherUser は、ほかのユーザーのアイテムを外さず、AppErrCodeResourceNotFound を返すことを検証する。
func TestDeleteItemUsecase_Execute_OtherUser(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	uc := usecase.NewDeleteItemUsecase(itemRepo)
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	itemID := testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()

	_, err := uc.Execute(t.Context(), usecase.DeleteItemInput{UserID: testutil.NewUserBuilder(t, tx).Build(), ItemID: itemID})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)
	if item, _ := itemRepo.FindByID(context.Background(), itemID); item.Status != model.ItemStatusListed {
		t.Errorf("ほかのユーザーのアイテム = %+v、リストに残すことを期待", item)
	}
}
