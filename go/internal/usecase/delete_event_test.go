package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestDeleteEventUsecase_Execute は、管理者だけがイベントを削除でき (編集者には AppErrCodeForbidden)、
// 版が違うときは AppErrCodeConflict を、削除したイベントをもう一度削除しようとすると
// AppErrCodeResourceNotFound を返すことを検証する。
func TestDeleteEventUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	repo := repository.NewEventRepository(db)
	uc := usecase.NewDeleteEventUsecase(db, validator.NewEventDeleteValidator(repository.NewItemRepository(db)), repo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	eventID := testutil.NewEventBuilder(t, db).Build()

	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.DeleteEventInput{User: newCommittedUserWithRole(t, model.UserRoleEditor), EventID: eventID}), model.AppErrCodeForbidden)

	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.DeleteEventInput{User: admin, EventID: eventID, LockVersion: 1}), model.AppErrCodeConflict)
	if found, _ := repo.FindByID(t.Context(), eventID); found.IsDeleted() {
		t.Error("版の違う送信でイベントを削除した")
	}

	if err := uc.Execute(t.Context(), usecase.DeleteEventInput{User: admin, EventID: eventID}); err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if found, _ := repo.FindByID(t.Context(), eventID); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}

	// 版ではなく削除した状態で拒むことを確かめるため、今の版を送る。
	assertAppErrorCode(t, uc.Execute(t.Context(), usecase.DeleteEventInput{User: admin, EventID: eventID, LockVersion: 1}), model.AppErrCodeResourceNotFound)
}

// TestDeleteEventUsecase_Execute_Referenced は、配下のグッズを参照するアイテムがあれば、
// リストから外したアイテムでも削除せず、アーカイブを案内する *model.ValidationError を返すことを検証する。
func TestDeleteEventUsecase_Execute_Referenced(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	repo := repository.NewEventRepository(db)
	uc := usecase.NewDeleteEventUsecase(db, validator.NewEventDeleteValidator(repository.NewItemRepository(db)), repo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	eventID := testutil.NewEventBuilder(t, db).Build()
	// 削除したカテゴリーの配下のグッズからの参照も数える。
	categoryID := testutil.NewEventCategoryBuilder(t, db, eventID).WithDeleted().Build()
	goodsID := testutil.NewGoodsBuilder(t, db, categoryID).Build()
	testutil.NewItemBuilder(t, db, admin.ID, goodsID).WithRemoved().Build()

	err := uc.Execute(jaContext(), usecase.DeleteEventInput{User: admin, EventID: eventID})
	if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
		t.Fatalf("エラー = %v、フォーム全体の *model.ValidationError を期待", err)
	}
	if found, _ := repo.FindByID(t.Context(), eventID); found.IsDeleted() {
		t.Error("アイテムから参照されているイベントを削除した")
	}
}
