package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestDeleteGoodsUsecase_Execute は、管理者だけがグッズを削除でき、戻り先のカテゴリーのIDを受け取れることと、
// 編集者には AppErrCodeForbidden を、古い版からの削除には AppErrCodeConflict を返すことを検証する。
func TestDeleteGoodsUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	eventRepo, categoryRepo, goodsRepo := repository.NewEventRepository(db), repository.NewEventCategoryRepository(db), repository.NewGoodsRepository(db)
	uc := usecase.NewDeleteGoodsUsecase(db, validator.NewGoodsDeleteValidator(repository.NewItemRepository(db)), eventRepo, categoryRepo, goodsRepo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	categoryID := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
	id := testutil.NewGoodsBuilder(t, db, categoryID).Build()

	_, err := uc.Execute(t.Context(), usecase.DeleteGoodsInput{User: newCommittedUserWithRole(t, model.UserRoleEditor), GoodsID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	_, err = uc.Execute(t.Context(), usecase.DeleteGoodsInput{User: admin, GoodsID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	output, err := uc.Execute(t.Context(), usecase.DeleteGoodsInput{User: admin, GoodsID: id})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.EventCategoryID != categoryID {
		t.Errorf("EventCategoryID = %s、期待値 = %s", output.EventCategoryID, categoryID)
	}
	if found, _ := goodsRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDeleteGoodsUsecase_Execute_Referenced は、グッズを参照するアイテムがあれば、
// リストから外したアイテムでも削除せず、アーカイブを案内する *model.ValidationError を返すことを検証する。
func TestDeleteGoodsUsecase_Execute_Referenced(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	eventRepo, categoryRepo, goodsRepo := repository.NewEventRepository(db), repository.NewEventCategoryRepository(db), repository.NewGoodsRepository(db)
	uc := usecase.NewDeleteGoodsUsecase(db, validator.NewGoodsDeleteValidator(repository.NewItemRepository(db)), eventRepo, categoryRepo, goodsRepo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	id := testutil.NewGoodsBuilder(t, db, testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()).Build()
	testutil.NewItemBuilder(t, db, admin.ID, id).WithRemoved().Build()

	_, err := uc.Execute(jaContext(), usecase.DeleteGoodsInput{User: admin, GoodsID: id})
	if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
		t.Fatalf("エラー = %v、フォーム全体の *model.ValidationError を期待", err)
	}
	if found, _ := goodsRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("アイテムから参照されているグッズを削除した")
	}
}
