package usecase_test

import (
	"context"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestUpdateItemUsecase_Execute は、アイテムの数量とひとことを更新し、戻り先のリストを決めるアイテムを返すことと、
// フォームの誤りでは更新しないことを検証する。
func TestUpdateItemUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	uc := usecase.NewUpdateItemUsecase(validator.NewItemUpdateValidator(), itemRepo)
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	itemID := testutil.NewItemBuilder(t, tx, userID, goodsID).WithKind(model.ItemKindWant).Build()

	_, err := uc.Execute(jaContext(), usecase.UpdateItemInput{UserID: userID, ItemID: itemID, Quantity: "0"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("quantity") {
		t.Errorf("数量0のエラー = %v、quantity の ValidationError を期待", err)
	}

	output, err := uc.Execute(jaContext(), usecase.UpdateItemInput{UserID: userID, ItemID: itemID, Quantity: "3", Note: "色違いでも可"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.Item.ID != itemID || output.Item.Kind != model.ItemKindWant {
		t.Errorf("結果のアイテム = %+v、ほしいリストの %s を期待", output.Item, itemID)
	}
	if item, _ := itemRepo.FindByID(context.Background(), itemID); item.Quantity != 3 || item.Note != "色違いでも可" {
		t.Errorf("更新後のアイテム = %+v、3点・「色違いでも可」を期待", item)
	}
	_, err = uc.Execute(jaContext(), usecase.UpdateItemInput{UserID: userID, ItemID: itemID, LockVersion: 0, Quantity: "1"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)
	if item, _ := itemRepo.FindByID(context.Background(), itemID); item.Quantity != 3 || item.Note != "色違いでも可" || item.LockVersion != 1 {
		t.Errorf("古い版からの更新後 = %+v、先の変更と版1を維持することを期待", item)
	}
}

// TestUpdateItemUsecase_Execute_NotFound は、外したアイテムとほかのユーザーのアイテムを更新せず、
// AppErrCodeResourceNotFound を返すことを検証する。
func TestUpdateItemUsecase_Execute_NotFound(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	itemRepo := repository.NewItemRepository(db).WithTx(tx)
	uc := usecase.NewUpdateItemUsecase(validator.NewItemUpdateValidator(), itemRepo)
	userID := testutil.NewUserBuilder(t, tx).Build()
	goodsID := testutil.NewGoodsBuilder(t, tx, newEventCategoryID(t, tx)).Build()
	otherItemID := testutil.NewItemBuilder(t, tx, testutil.NewUserBuilder(t, tx).Build(), goodsID).Build()

	for name, itemID := range map[string]model.ItemID{
		"外したアイテム":      testutil.NewItemBuilder(t, tx, userID, goodsID).WithRemoved().Build(),
		"ほかのユーザーのアイテム": otherItemID,
	} {
		_, err := uc.Execute(jaContext(), usecase.UpdateItemInput{UserID: userID, ItemID: itemID, Quantity: "5"})
		if ae := model.AsAppError(err); ae == nil || ae.Code != model.AppErrCodeResourceNotFound {
			t.Errorf("%s: エラー = %v、AppErrCodeResourceNotFound を期待", name, err)
		}
	}
	if item, _ := itemRepo.FindByID(context.Background(), otherItemID); item.Quantity != 1 {
		t.Errorf("ほかのユーザーのアイテム = %+v、数量を変えないことを期待", item)
	}
}
