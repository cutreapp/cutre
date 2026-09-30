package usecase_test

import (
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// TestDeleteEventCategoryUsecase_Execute は、管理者だけがカテゴリーを削除でき、戻り先のイベントのIDを受け取れることと、
// 編集者には AppErrCodeForbidden を、古い版からの削除には AppErrCodeConflict を返すことを検証する。
func TestDeleteEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	categoryRepo := repository.NewEventCategoryRepository(db)
	uc := usecase.NewDeleteEventCategoryUsecase(db, validator.NewEventCategoryDeleteValidator(repository.NewItemRepository(db)), repository.NewEventRepository(db), categoryRepo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	eventID := testutil.NewEventBuilder(t, db).Build()
	id := testutil.NewEventCategoryBuilder(t, db, eventID).Build()

	_, err := uc.Execute(t.Context(), usecase.DeleteEventCategoryInput{User: newCommittedUserWithRole(t, model.UserRoleEditor), EventCategoryID: id})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	_, err = uc.Execute(t.Context(), usecase.DeleteEventCategoryInput{User: admin, EventCategoryID: id, LockVersion: 1})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	output, err := uc.Execute(t.Context(), usecase.DeleteEventCategoryInput{User: admin, EventCategoryID: id})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.EventID != eventID {
		t.Errorf("EventID = %s、期待値 = %s", output.EventID, eventID)
	}
	if found, _ := categoryRepo.FindByID(t.Context(), id); !found.IsDeleted() {
		t.Errorf("状態 = %q、削除した状態を期待", found.Status)
	}
}

// TestDeleteEventCategoryUsecase_Execute_Referenced は、配下のグッズを参照するアイテムがあれば、
// リストから外したアイテムでも削除せず、アーカイブを案内する *model.ValidationError を返すことを検証する。
func TestDeleteEventCategoryUsecase_Execute_Referenced(t *testing.T) {
	t.Parallel()

	db := testutil.GetTestDB()
	categoryRepo := repository.NewEventCategoryRepository(db)
	uc := usecase.NewDeleteEventCategoryUsecase(db, validator.NewEventCategoryDeleteValidator(repository.NewItemRepository(db)), repository.NewEventRepository(db), categoryRepo)
	admin := newCommittedUserWithRole(t, model.UserRoleAdmin)
	id := testutil.NewEventCategoryBuilder(t, db, testutil.NewEventBuilder(t, db).Build()).Build()
	goodsID := testutil.NewGoodsBuilder(t, db, id).WithArchived("景品から外れたため").Build()
	testutil.NewItemBuilder(t, db, admin.ID, goodsID).WithRemoved().Build()

	_, err := uc.Execute(jaContext(), usecase.DeleteEventCategoryInput{User: admin, EventCategoryID: id})
	if ve := model.AsValidationError(err); ve == nil || len(ve.Global) != 1 {
		t.Fatalf("エラー = %v、フォーム全体の *model.ValidationError を期待", err)
	}
	if found, _ := categoryRepo.FindByID(t.Context(), id); found.IsDeleted() {
		t.Error("アイテムから参照されているカテゴリーを削除した")
	}
}
