package usecase_test

import (
	"database/sql"
	"testing"

	"github.com/cutreapp/cutre/go/internal/model"
	"github.com/cutreapp/cutre/go/internal/repository"
	"github.com/cutreapp/cutre/go/internal/testutil"
	"github.com/cutreapp/cutre/go/internal/usecase"
	"github.com/cutreapp/cutre/go/internal/validator"
)

// newUpdateEventCategoryUsecase はテストのトランザクションの中で動く UpdateEventCategoryUsecase を作る。
func newUpdateEventCategoryUsecase(db *sql.DB, tx *sql.Tx) *usecase.UpdateEventCategoryUsecase {
	return usecase.NewUpdateEventCategoryUsecase(
		validator.NewEventCategoryUpdateValidator(),
		repository.NewEventRepository(db).WithTx(tx),
		repository.NewEventCategoryRepository(db).WithTx(tx),
	)
}

// TestUpdateEventCategoryUsecase_Execute は、編集者がカテゴリーを更新して戻り先のイベントのIDを受け取れ、
// 古い版からの更新には AppErrCodeConflict を返して上書きしないことを検証する。
func TestUpdateEventCategoryUsecase_Execute(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newUpdateEventCategoryUsecase(db, tx)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	eventID := testutil.NewEventBuilder(t, tx).Build()
	id := testutil.NewEventCategoryBuilder(t, tx, eventID).Build()

	output, err := uc.Execute(t.Context(), usecase.UpdateEventCategoryInput{User: editor, EventCategoryID: id, Name: "B賞", Position: "2"})
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}
	if output.EventID != eventID {
		t.Errorf("EventID = %s、期待値 = %s", output.EventID, eventID)
	}

	_, err = uc.Execute(t.Context(), usecase.UpdateEventCategoryInput{User: editor, EventCategoryID: id, Name: "古い版からのカテゴリー", Position: "3"})
	assertAppErrorCode(t, err, model.AppErrCodeConflict)

	found, _ := repository.NewEventCategoryRepository(db).WithTx(tx).FindByID(t.Context(), id)
	if found.Name != "B賞" || found.Position != 2 || found.LockVersion != 1 {
		t.Errorf("カテゴリー = %+v、「B賞」・並び順2・版1を期待", found)
	}
}

// TestUpdateEventCategoryUsecase_Execute_Errors は、一般のユーザーには AppErrCodeForbidden を、
// 削除したイベントのカテゴリーには AppErrCodeResourceNotFound を、フォームの誤りには *model.ValidationError を返すことを検証する。
func TestUpdateEventCategoryUsecase_Execute_Errors(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	uc := newUpdateEventCategoryUsecase(db, tx)
	editor := newUserWithRole(t, tx, model.UserRoleEditor)
	id := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).Build()).Build()

	_, err := uc.Execute(t.Context(), usecase.UpdateEventCategoryInput{User: newUserWithRole(t, tx, model.UserRoleUser), EventCategoryID: id, Name: "B賞", Position: "2"})
	assertAppErrorCode(t, err, model.AppErrCodeForbidden)

	deletedEventCategoryID := testutil.NewEventCategoryBuilder(t, tx, testutil.NewEventBuilder(t, tx).WithDeleted().Build()).Build()
	_, err = uc.Execute(t.Context(), usecase.UpdateEventCategoryInput{User: editor, EventCategoryID: deletedEventCategoryID, Name: "B賞", Position: "2"})
	assertAppErrorCode(t, err, model.AppErrCodeResourceNotFound)

	_, err = uc.Execute(jaContext(), usecase.UpdateEventCategoryInput{User: editor, EventCategoryID: id, Name: "", Position: "2"})
	if ve := model.AsValidationError(err); ve == nil || !ve.HasFieldError("name") {
		t.Errorf("名前が空のエラー = %v、name の ValidationError を期待", err)
	}
}
